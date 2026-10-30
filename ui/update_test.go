package ui

import "testing"

func TestNewerVersion(t *testing.T) {
	if !newerVersion("0.2.0", "0.2.1") || !newerVersion("v0.2.0", "v0.3.0") {
		t.Fatal("较新的版本没有被认出来")
	}
	if newerVersion("0.2.0", "0.2.0") || newerVersion("0.3.0", "0.2.9") {
		t.Fatal("相同或更旧的版本不应提示更新")
	}
}

func TestAllowedSetupURL(t *testing.T) {
	ok := "https://github.com/SwalkTech/Skiff/releases/download/v0.2.1/Skiff-Setup.exe"
	if !allowedSetupURL(ok) {
		t.Fatal("正式安装包地址应被允许")
	}
	for _, raw := range []string{
		"http://github.com/SwalkTech/Skiff/releases/download/v0.2.1/Skiff-Setup.exe",
		"https://example.com/Skiff-Setup.exe",
		"https://github.com/other/Skiff/releases/download/v0.2.1/Skiff-Setup.exe",
		"https://github.com/SwalkTech/Skiff/releases/download/v0.2.1/other.exe",
	} {
		if allowedSetupURL(raw) {
			t.Fatalf("不应允许 %s", raw)
		}
	}
}

func TestParseLatestRelease(t *testing.T) {
	body := []byte(`{"tag_name":"v0.2.1","assets":[{"name":"Skiff-Setup.exe","browser_download_url":"https://github.com/SwalkTech/Skiff/releases/download/v0.2.1/Skiff-Setup.exe"}]}`)
	version, rawURL, err := parseLatestRelease(body)
	if err != nil || version != "0.2.1" || !allowedSetupURL(rawURL) {
		t.Fatalf("version=%s url=%s err=%v", version, rawURL, err)
	}
}

func TestUpdatePromptAsks(t *testing.T) {
	model := &Model{Selected: -1, Status: "未连接", updateVersion: "9.9.9"}
	driver := newDesk(t, model)
	if !hasLabel(driver.Snapshot(), "发现新版本 9.9.9，是否更新到最新？") {
		t.Fatal("有新版本时没有询问")
	}
	clickID(t, driver, updateNoID)
	if hasLabel(driver.Snapshot(), "发现新版本") {
		t.Fatal("选择稍后之后仍在询问")
	}
}
