//go:build windows

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func applyUpdate(setupPath string) error {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		fmt.Sprintf("Wait-Process -Id %d -ErrorAction SilentlyContinue; Start-Process -FilePath '%s'", os.Getpid(), escapePowerShell(setupPath)))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd.Start()
}

func escapePowerShell(path string) string {
	return stringsReplace(path, "'", "''")
}

func stringsReplace(s, old, new string) string {
	out := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out += new
			i += len(old)
			continue
		}
		out += s[i : i+1]
		i++
	}
	return out
}
