package ui

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/SwalkTech/Skiff/session"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func TestSFTPUploadDownloadAndList(t *testing.T) {
	remote := filepath.ToSlash(t.TempDir())
	addr := startSFTPServer(t)
	ctx := t.Context()
	client, err := session.Dial(ctx, addr, session.Config{
		User: "alice",
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error {
			return nil
		},
		Password: func() (string, error) { return "secret-password", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	raw, err := client.Conn()
	if err != nil {
		t.Fatal(err)
	}
	files, err := sftp.NewClient(raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { files.Close() })

	local := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(local, []byte("skiff-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := uploadFile(files, remote, local); err != nil {
		t.Fatal(err)
	}
	infos, err := files.ReadDir(remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].Name() != "note.txt" {
		t.Fatalf("远程目录 = %#v", names(infos))
	}
	dest := filepath.Join(t.TempDir(), "down.txt")
	if err := downloadFile(files, remote+"/note.txt", dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "skiff-file" {
		t.Fatalf("下载内容 = %q", got)
	}
}

func names(infos []os.FileInfo) []string {
	out := make([]string, len(infos))
	for i, info := range infos {
		out[i] = info.Name()
	}
	return out
}

func startSFTPServer(t *testing.T) string {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == "alice" && string(password) == "secret-password" {
				return nil, nil
			}
			return nil, errors.New("认证失败")
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveSFTP(conn, config)
		}
	}()
	return listener.Addr().String()
}

func serveSFTP(conn net.Conn, config *ssh.ServerConfig) {
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
				if req.Type != "subsystem" || !isSFTP(req.Payload) {
					if req.WantReply {
						req.Reply(false, nil)
					}
					continue
				}
				if req.WantReply {
					req.Reply(true, nil)
				}
				server, err := sftp.NewServer(channel)
				if err != nil {
					return
				}
				_ = server.Serve()
				return
			}
		}()
	}
}

func isSFTP(payload []byte) bool {
	var msg struct{ Name string }
	return ssh.Unmarshal(payload, &msg) == nil && msg.Name == "sftp"
}
