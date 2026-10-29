package ui

import (
	"fmt"
	"strings"

	"github.com/SwalkTech/Skiff/store"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	vivaui "github.com/vivaui/viva/ui"
)

var (
	tree = vivaui.Tree{}
	page = tree.Feature("desk")
)

// Model 是主窗口要画的内容。它不保存连接，也不发起 SSH 握手。
type Model struct {
	Hosts  []store.Host
	Screen string
	view   flex.View
}

// NewConfig 描述主机列表和一块静态等宽文本。测试可以驱动同一份配置而不打开窗口。
func NewConfig(m *Model) app.Config {
	theme := vivaui.LightTheme()
	config := app.Config{Tree: &tree, Title: "Skiff", Width: 800, Height: 480}
	config.Paint = func(canvas *vivaui.Canvas, size vivaui.Size) {
		bounds := vivaui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)
		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Column().Fill().Pad(16).Gap(8).Children(func() {
				f.Text("主机").Bold()
				if len(m.Hosts) == 0 {
					f.Text("还没有主机").Muted()
				}
				for _, host := range m.Hosts {
					f.Text(hostLabel(host))
				}
				f.Text("终端").Bold()
				for _, line := range screenLines(m.Screen) {
					f.Text(line).Mono()
				}
			})
		})
	}
	return config
}

// Load 从主机清单文件构造窗口模型。文件不存在时列表为空。
// 读到的地址、用户和端口会原样显示，口令不在清单中。
func Load(path string) (*Model, error) {
	hosts, err := store.Open(path).List()
	if err != nil {
		return nil, err
	}
	return &Model{Hosts: hosts}, nil
}

// Run 打开主窗口。调用方负责准备要显示的主机和文本。
func Run(m *Model) error {
	var canvas vivaui.Canvas
	return app.RunCanvas(NewConfig(m), &canvas)
}

func hostLabel(host store.Host) string {
	return fmt.Sprintf("%s@%s:%d", host.User, host.Address, host.Port)
}

func screenLines(screen string) []string {
	if strings.TrimSpace(screen) == "" {
		return []string{"等待连接"}
	}
	return strings.Split(screen, "\n")
}
