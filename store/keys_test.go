package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRememberAndTrustHostKey(t *testing.T) {
	path := KeysPath(filepath.Join(t.TempDir(), "hosts.json"))
	keys := OpenKeys(path)
	const authorized = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey"
	if err := keys.Remember("example.com", 22, authorized); err != nil {
		t.Fatal(err)
	}
	got, ok := OpenKeys(path).Trusted("example.com", 22)
	if !ok || got != authorized {
		t.Fatalf("读回的密钥 = %q, ok=%v", got, ok)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" || contains(raw, []byte("password")) {
		t.Fatalf("密钥文件内容异常: %s", raw)
	}
}

func contains(haystack, needle []byte) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle []byte) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
