package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactSecretLeavesEmptyPasswordUntouched(t *testing.T) {
	got := redactSecret("认证失败", "")
	if got != "认证失败" {
		t.Fatalf("空口令把错误改成了 %q", got)
	}
}

func TestPublishDialErrRedactsPassword(t *testing.T) {
	model := &Model{generation: 1}
	model.publishDialErr(1, errors.New("认证失败: secret-password"), "secret-password")
	if strings.Contains(model.dialErr, "secret-password") {
		t.Fatalf("错误信息仍包含口令: %q", model.dialErr)
	}
	if model.dialErr != "认证失败: ******" {
		t.Fatalf("打码结果 = %q", model.dialErr)
	}
}

func TestPublishConnErrRedactsPassword(t *testing.T) {
	model := &Model{}
	session := &conn{generation: 3}
	model.publishConnErr(session, 3, errors.New("认证失败: secret-password"), "")
	if session.dialErr != "认证失败: secret-password" {
		t.Fatalf("空口令把分栏错误改成了 %q", session.dialErr)
	}
	model.publishConnErr(session, 3, errors.New("认证失败: secret-password"), "secret-password")
	if strings.Contains(session.dialErr, "secret-password") {
		t.Fatalf("分栏错误仍包含口令: %q", session.dialErr)
	}
}
