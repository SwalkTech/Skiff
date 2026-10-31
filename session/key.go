package session

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// SignerFromPEM 解析 OpenSSH 或 PEM 私钥。passphrase 为空时按未加密私钥处理。
// RSA、ECDSA 和 Ed25519 都可以。调用方必须在主机密钥被接受之后才调用它。
func SignerFromPEM(data []byte, passphrase string) (ssh.Signer, error) {
	var (
		key any
		err error
	)
	if passphrase == "" {
		key, err = ssh.ParsePrivateKey(data)
	} else {
		key, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
	}
	if err != nil {
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return nil, errors.New("session: 私钥需要口令")
		}
		return nil, fmt.Errorf("session: 无法读取私钥: %w", err)
	}
	signer, ok := key.(ssh.Signer)
	if !ok || signer == nil {
		return nil, errors.New("session: 无法使用私钥")
	}
	return signer, nil
}

// SignersFromFile 读取私钥文件并返回一个签名器。它会立刻读文件，只能放在认证回调里。
func SignersFromFile(path, passphrase string) ([]ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session: 无法打开私钥文件: %w", err)
	}
	signer, err := SignerFromPEM(data, passphrase)
	if err != nil {
		return nil, err
	}
	return []ssh.Signer{signer}, nil
}
