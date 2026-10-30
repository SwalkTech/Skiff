package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func main() {
	if len(payload) == 0 {
		alert("安装失败", "这个安装器没有包含 Skiff.exe。\n请使用带 installer 标签的发布构建。")
		return
	}
	dir, err := installDir()
	if err != nil {
		alert("安装失败", err.Error())
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		alert("安装失败", err.Error())
		return
	}
	dest := filepath.Join(dir, "Skiff.exe")
	if err := os.WriteFile(dest, payload, 0o755); err != nil {
		alert("安装失败", "无法写入 "+dest+"\n请先关闭正在运行的 Skiff。\n"+err.Error())
		return
	}
	if err := createShortcut(dest); err != nil {
		alert("已安装", "程序已放到 "+dest+"，但开始菜单快捷方式没有建成。\n"+err.Error())
	} else {
		alert("安装完成", "Skiff 已安装。\n"+dest+"\n开始菜单里可以找到它。")
	}
	_ = exec.Command(dest).Start()
}

func installDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "Skiff"), nil
}

func createShortcut(target string) error {
	menu, err := startMenu()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(menu, 0o755); err != nil {
		return err
	}
	desktop, err := desktopDir()
	if err != nil {
		desktop = ""
	}
	script := "$shell = New-Object -ComObject WScript.Shell; " +
		"$link = $shell.CreateShortcut(" + psQuote(filepath.Join(menu, "Skiff.lnk")) + "); " +
		"$link.TargetPath = " + psQuote(target) + "; " +
		"$link.WorkingDirectory = " + psQuote(filepath.Dir(target)) + "; " +
		"$link.Description = 'Skiff SSH 客户端'; " +
		"$link.Save(); "
	if desktop != "" {
		script += "$desk = $shell.CreateShortcut(" + psQuote(filepath.Join(desktop, "Skiff.lnk")) + "); " +
			"$desk.TargetPath = " + psQuote(target) + "; " +
			"$desk.WorkingDirectory = " + psQuote(filepath.Dir(target)) + "; " +
			"$desk.Description = 'Skiff SSH 客户端'; " +
			"$desk.Save()"
	}
	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd.Run()
}

func startMenu() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Microsoft", "Windows", "Start Menu", "Programs"), nil
}

func desktopDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Desktop"), nil
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func alert(title, text string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	box := user32.NewProc("MessageBoxW")
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	_, _, _ = box.Call(0, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), 0x40)
}
