//go:build !windows

package ui

import "errors"

func applyUpdate(string) error {
	return errors.New("当前系统不支持在线更新")
}
