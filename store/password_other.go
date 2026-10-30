//go:build !windows

package store

// SystemPasswords 在非 Windows 上不保存口令。
func SystemPasswords() PasswordStore { return nopPasswords{} }

type nopPasswords struct{}

func (nopPasswords) Load(string, string, int) (string, bool) { return "", false }
func (nopPasswords) Save(string, string, int, string) error  { return nil }
func (nopPasswords) Delete(string, string, int) error        { return nil }
