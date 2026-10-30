//go:build windows

package store

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
)

type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

var (
	advapi32       = windows.NewLazySystemDLL("advapi32.dll")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

// SystemPasswords 使用 Windows 凭据管理器保存口令。
func SystemPasswords() PasswordStore { return windowsPasswords{} }

type windowsPasswords struct{}

func (windowsPasswords) Load(address, user string, port int) (string, bool) {
	target, err := windows.UTF16PtrFromString(PasswordTarget(address, user, port))
	if err != nil {
		return "", false
	}
	var raw uintptr
	r, _, _ := procCredRead.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&raw)))
	if r == 0 || raw == 0 {
		return "", false
	}
	defer procCredFree.Call(raw)
	cred := (*credential)(unsafe.Pointer(raw))
	if cred.CredentialBlob == nil || cred.CredentialBlobSize == 0 {
		return "", false
	}
	blob := unsafe.Slice(cred.CredentialBlob, cred.CredentialBlobSize)
	password := strings.TrimRight(string(blob), "\x00")
	if password == "" {
		return "", false
	}
	return password, true
}

func (windowsPasswords) Save(address, user string, port int, password string) error {
	if strings.TrimSpace(password) == "" {
		return nil
	}
	target, err := windows.UTF16PtrFromString(PasswordTarget(address, user, port))
	if err != nil {
		return err
	}
	userName, err := windows.UTF16PtrFromString(user)
	if err != nil {
		return err
	}
	blob := append([]byte(password), 0)
	cred := credential{
		Type:               credTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     &blob[0],
		Persist:            credPersistLocalMachine,
		UserName:           userName,
	}
	r, _, callErr := procCredWrite.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if r == 0 {
		return callErr
	}
	return nil
}

func (windowsPasswords) Delete(address, user string, port int) error {
	target, err := windows.UTF16PtrFromString(PasswordTarget(address, user, port))
	if err != nil {
		return err
	}
	r, _, callErr := procCredDelete.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0)
	if r == 0 && callErr != windows.ERROR_NOT_FOUND {
		return callErr
	}
	return nil
}
