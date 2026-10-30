//go:build !installer

package main

// payload 在日常构建中为空。发布安装器时使用 -tags installer，并在本目录放好 Skiff.exe。
var payload []byte
