package session

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestDialRejectsHostKeyBeforeAuth(t *testing.T) {
	var passwordReads atomic.Int32
	var serverAuth atomic.Int32
	addr, _ := startTestServer(t, &serverAuth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := Dial(ctx, addr, Config{
		User: "alice",
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error {
			return errors.New("主机密钥不匹配")
		},
		Password: func() (string, error) {
			passwordReads.Add(1)
			return "secret", nil
		},
	})
	if err == nil {
		t.Fatal("主机密钥不匹配时仍然连接成功")
	}
	if passwordReads.Load() != 0 {
		t.Fatalf("主机密钥被拒绝后读取了 %d 次口令", passwordReads.Load())
	}
	if serverAuth.Load() != 0 {
		t.Fatalf("主机密钥被拒绝后服务器收到了 %d 次认证", serverAuth.Load())
	}
}

func TestPTYClosesChannel(t *testing.T) {
	var serverAuth atomic.Int32
	addr, signer := startTestServer(t, &serverAuth)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var passwordReads atomic.Int32
	client, err := Dial(ctx, addr, Config{
		User:            "alice",
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
		Password: func() (string, error) {
			passwordReads.Add(1)
			return "secret", nil
		},
	})
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	if passwordReads.Load() == 0 {
		t.Fatal("主机密钥接受后没有读取口令")
	}

	if err := client.StartPTY(80, 24); err != nil {
		t.Fatalf("启动 PTY 失败: %v", err)
	}
	if err := client.Resize(100, 30); err != nil {
		t.Fatalf("调整 PTY 尺寸失败: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("关闭会话失败: %v", err)
	}
	if _, err := client.Write([]byte("echo\n")); !errors.Is(err, ErrClosed) {
		t.Fatalf("关闭后的写入错误 = %v", err)
	}
}

func startTestServer(t *testing.T, authCount *atomic.Int32) (string, ssh.Signer) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成主机密钥失败: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("转换主机密钥失败: %v", err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			authCount.Add(1)
			if conn.User() == "alice" && string(password) == "secret" {
				return nil, nil
			}
			return nil, errors.New("认证失败")
		},
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveTestConn(conn, config)
		}
	}()
	return listener.Addr().String(), signer
}

func serveTestConn(conn net.Conn, config *ssh.ServerConfig) {
	serverConn, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer serverConn.Close()
	go ssh.DiscardRequests(requests)
	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "不支持的通道")
			continue
		}
		channel, reqs, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for req := range reqs {
				switch req.Type {
				case "pty-req", "shell", "window-change":
					if req.WantReply {
						req.Reply(true, nil)
					}
				default:
					if req.WantReply {
						req.Reply(false, nil)
					}
				}
			}
		}()
	}
}
