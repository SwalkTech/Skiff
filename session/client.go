package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// ErrClosed 表示会话已经关闭，不能再读写。
var ErrClosed = errors.New("session: 会话已关闭")

// Config 是一次 SSH 连接的参数。
//
// Password 和 Signers 只会在主机密钥被接受之后调用。密钥被拒绝时，连接在用户认证之前结束。
type Config struct {
	User            string
	HostKeyCallback ssh.HostKeyCallback
	Password        func() (string, error)
	Signers         func() ([]ssh.Signer, error)
}

// Client 是一条已通过主机密钥校验的 SSH 连接，最多承载一个远端 PTY。
type Client struct {
	mu      sync.Mutex
	raw     *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
	closed  bool
}

// Dial 建立 SSH 连接。主机密钥回调返回错误时，不会读取口令或私钥。
func Dial(ctx context.Context, address string, cfg Config) (*Client, error) {
	if cfg.HostKeyCallback == nil {
		return nil, errors.New("session: 缺少主机密钥校验")
	}
	if cfg.Password == nil && cfg.Signers == nil {
		return nil, errors.New("session: 未配置认证方式")
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		HostKeyCallback: cfg.HostKeyCallback,
	}
	if cfg.Signers != nil {
		sshCfg.Auth = append(sshCfg.Auth, ssh.PublicKeysCallback(cfg.Signers))
	}
	if cfg.Password != nil {
		sshCfg.Auth = append(sshCfg.Auth, ssh.PasswordCallback(cfg.Password))
		sshCfg.Auth = append(sshCfg.Auth, ssh.KeyboardInteractive(func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
			if len(questions) == 0 {
				return nil, nil
			}
			password, err := cfg.Password()
			if err != nil {
				return nil, err
			}
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = password
			}
			return answers, nil
		}))
	}

	dialer := net.Dialer{Timeout: 15 * time.Second}
	netConn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	conn, chans, reqs, err := ssh.NewClientConn(netConn, address, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("session: 连接失败: %w", err)
	}
	return &Client{raw: ssh.NewClient(conn, chans, reqs)}, nil
}

// StartPTY 请求远端 shell 和 PTY。cols 和 rows 是字符列数和行数。
func (c *Client) StartPTY(cols, rows int) error {
	if cols < 1 || rows < 1 {
		return errors.New("session: PTY 尺寸必须为正数")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClosed
	}
	if c.session != nil {
		return errors.New("session: PTY 已经启动")
	}

	session, err := c.raw.NewSession()
	if err != nil {
		return err
	}
	session.Stderr = io.Discard
	if err := session.RequestPty("xterm-256color", rows, cols, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		session.Close()
		return err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return err
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return err
	}
	if err := session.Shell(); err != nil {
		session.Close()
		return err
	}
	c.session = session
	c.stdin = stdin
	c.stdout = stdout
	return nil
}

// Resize 通知远端 PTY 新的字符尺寸。
func (c *Client) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 {
		return errors.New("session: PTY 尺寸必须为正数")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.session == nil {
		return ErrClosed
	}
	return c.session.WindowChange(rows, cols)
}

// Read 读取远端 PTY 的输出。
func (c *Client) Read(p []byte) (int, error) {
	c.mu.Lock()
	if c.closed || c.stdout == nil {
		c.mu.Unlock()
		return 0, ErrClosed
	}
	stdout := c.stdout
	c.mu.Unlock()
	return stdout.Read(p)
}

// Write 向远端 PTY 写入输入。会话关闭后返回 ErrClosed。
func (c *Client) Write(p []byte) (int, error) {
	c.mu.Lock()
	if c.closed || c.stdin == nil {
		c.mu.Unlock()
		return 0, ErrClosed
	}
	stdin := c.stdin
	c.mu.Unlock()
	return stdin.Write(p)
}

// Output 在同一条 SSH 连接上执行一条命令并返回标准输出。它不占用已经打开的 PTY。
func (c *Client) Output(command string) ([]byte, error) {
	c.mu.Lock()
	if c.closed || c.raw == nil {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	raw := c.raw
	c.mu.Unlock()
	session, err := raw.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.Output(command)
}

// Conn 返回底层 SSH 连接，供 SFTP 等子系统复用。会话关闭后返回 ErrClosed。
func (c *Client) Conn() (*ssh.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.raw == nil {
		return nil, ErrClosed
	}
	return c.raw, nil
}

// Close 关闭 PTY 和 SSH 连接。重复关闭不会再次访问通道。
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	var err error
	if c.session != nil {
		err = c.session.Close()
	}
	if c.raw != nil {
		if closeErr := c.raw.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}
