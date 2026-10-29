// Package ui 绘制主机列表和终端单元格。
//
// 界面把按键、尺寸变化和连接请求交给外部处理，不实现 SSH 握手。后续窗口依赖
// 固定为已发布的 github.com/vivaui/viva v0.2.0，不使用本地 replace。
package ui
