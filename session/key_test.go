package session

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestDialRejectsHostKeyBeforeSigner(t *testing.T) {
	var signerReads atomic.Int32
	var serverAuth atomic.Int32
	addr, _ := startTestServer(t, &serverAuth)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Dial(ctx, addr, Config{
		User: "alice",
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error {
			return errors.New("主机密钥不匹配")
		},
		Signers: func() ([]ssh.Signer, error) {
			signerReads.Add(1)
			return nil, errors.New("不应读取私钥")
		},
	})
	if err == nil {
		t.Fatal("主机密钥不匹配时仍然连接成功")
	}
	if signerReads.Load() != 0 {
		t.Fatalf("主机密钥被拒绝后读取了 %d 次私钥", signerReads.Load())
	}
	if serverAuth.Load() != 0 {
		t.Fatalf("主机密钥被拒绝后服务器收到了 %d 次认证", serverAuth.Load())
	}
}

func TestDialUsesPrivateKeyFile(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成 RSA 私钥失败: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("转换私钥失败: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(key, "skiff")
	if err != nil {
		t.Fatalf("编码私钥失败: %v", err)
	}
	path := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("写入私钥失败: %v", err)
	}

	var serverAuth atomic.Int32
	addr := startKeyServer(t, signer.PublicKey(), &serverAuth)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var fileReads atomic.Int32
	client, err := Dial(ctx, addr, Config{
		User:            "alice",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Signers: func() ([]ssh.Signer, error) {
			fileReads.Add(1)
			return SignersFromFile(path, "")
		},
	})
	if err != nil {
		t.Fatalf("私钥登录失败: %v", err)
	}
	client.Close()
	if fileReads.Load() == 0 {
		t.Fatal("主机密钥接受后没有读取私钥文件")
	}
	if serverAuth.Load() == 0 {
		t.Fatal("服务器没有收到公钥认证")
	}
}

func TestSignerFromPEMAcceptsRSAAndPassphrase(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成 RSA 私钥失败: %v", err)
	}
	plain, err := ssh.MarshalPrivateKey(key, "skiff")
	if err != nil {
		t.Fatalf("编码私钥失败: %v", err)
	}
	signer, err := SignerFromPEM(pem.EncodeToMemory(plain), "")
	if err != nil {
		t.Fatalf("未加密 RSA 私钥无法使用: %v", err)
	}
	if signer.PublicKey().Type() != ssh.KeyAlgoRSA {
		t.Fatalf("公钥类型 = %s", signer.PublicKey().Type())
	}

	locked, err := ssh.MarshalPrivateKeyWithPassphrase(key, "skiff", []byte("phrase"))
	if err != nil {
		t.Fatalf("加密私钥失败: %v", err)
	}
	if _, err := SignerFromPEM(pem.EncodeToMemory(locked), ""); err == nil {
		t.Fatal("加密私钥在没有口令时仍然解析成功")
	}
	if _, err := SignerFromPEM(pem.EncodeToMemory(locked), "phrase"); err != nil {
		t.Fatalf("带口令的私钥无法使用: %v", err)
	}
}

func TestSignerFromPEMAcceptsEd25519(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成私钥失败: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(key, "skiff")
	if err != nil {
		t.Fatalf("编码私钥失败: %v", err)
	}
	if _, err := SignerFromPEM(pem.EncodeToMemory(block), ""); err != nil {
		t.Fatalf("Ed25519 私钥无法使用: %v", err)
	}
}

func startKeyServer(t *testing.T, accepted ssh.PublicKey, authCount *atomic.Int32) string {
	t.Helper()
	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成主机密钥失败: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatalf("转换主机密钥失败: %v", err)
	}
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			authCount.Add(1)
			if conn.User() == "alice" && string(key.Marshal()) == string(accepted.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("认证失败")
		},
	}
	config.AddHostKey(hostSigner)
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
	return listener.Addr().String()
}
