package store

import "fmt"

// PasswordStore 保存主机口令。实现不得把口令写入主机清单。
type PasswordStore interface {
	Load(address, user string, port int) (string, bool)
	Save(address, user string, port int, password string) error
	Delete(address, user string, port int) error
}

// PasswordTarget 是系统密钥库里的目标名。
func PasswordTarget(address, user string, port int) string {
	return fmt.Sprintf("Skiff:%s@%s:%d", user, address, port)
}
