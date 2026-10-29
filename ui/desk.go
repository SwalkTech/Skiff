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
	Hosts []store.Host
	// Selected 是当前主机在 Hosts 中的下标。没有可选主机时为 -1。
	Selected int
	Screen   string
	Status   string
	view     flex.View
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
				for i, host := range m.Hosts {
					label := hostLabel(host, i == m.Selected)
					text := f.Text(label)
					if i == m.Selected {
						text.Bold()
					}
				}
				f.Text("终端").Bold()
				for _, line := range screenLines(m.Screen) {
					f.Text(line).Mono()
				}
				f.Spacer()
				f.Text(statusText(m.Status)).Muted()
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
	selected := -1
	if len(hosts) > 0 {
		selected = 0
	}
	return &Model{Hosts: hosts, Selected: selected, Status: "未连接"}, nil
}

// Fail 把失败原因写到状态栏。窗口保持打开，原因中不应包含口令。
func (m *Model) Fail(reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "连接失败"
	}
	m.Status = reason
}

// Run 打开主窗口。调用方负责准备要显示的主机和文本。
func Run(m *Model) error {
	var canvas vivaui.Canvas
	return app.RunCanvas(NewConfig(m), &canvas)
}

func hostLabel(host store.Host, selected bool) string {
	label := fmt.Sprintf("%s@%s:%d", host.User, host.Address, host.Port)
	if selected {
		return "当前 " + label
	}
	return label
}

func screenLines(screen string) []string {
	if strings.TrimSpace(screen) == "" {
		return []string{"等待连接"}
	}
	return strings.Split(screen, "\n")
}

func statusText(status string) string {
	if strings.TrimSpace(status) == "" {
		return "未连接"
	}
	return status
}
