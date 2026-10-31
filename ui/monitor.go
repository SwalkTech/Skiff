package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/flex"
)

const loadScript = `echo LOAD; head -n 1 /proc/loadavg; echo MEM; grep -E '^(MemTotal|MemAvailable|SwapTotal|SwapFree):' /proc/meminfo; echo CPU; head -n 1 /proc/stat; echo NET; cat /proc/net/dev; echo UP; head -n 1 /proc/uptime; echo DISK; df -B1 / | tail -n 1`

type serverLoad struct {
	CPU       float64
	MemUsed   uint64
	MemTotal  uint64
	DiskUsed  uint64
	DiskTotal uint64
	Load1     float64
	Load5     float64
	Load15    float64
	Rx        uint64
	Tx        uint64
	RxRate    float64
	TxRate    float64
	Uptime    float64
}

type cpuSample struct {
	total uint64
	idle  uint64
	when  time.Time
}

type netSample struct {
	rx, tx uint64
	when   time.Time
}

func (m *Model) watchLoad(ctx context.Context, gen int) {
	var cpu cpuSample
	var net netSample
	m.sampleLoad(ctx, gen, &cpu, &net)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !m.sampleLoad(ctx, gen, &cpu, &net) {
				return
			}
		}
	}
}

func (m *Model) sampleLoad(ctx context.Context, gen int, cpu *cpuSample, net *netSample) bool {
	if ctx.Err() != nil {
		return false
	}
	m.mu.Lock()
	current := m.generation == gen && m.client != nil
	client := m.client
	m.mu.Unlock()
	if !current {
		return false
	}
	out, err := client.Output(loadScript)
	if err != nil {
		m.publishLoad(serverLoad{}, "无法读取负载")
		return ctx.Err() == nil && m.generationAlive(gen)
	}
	load, nextCPU, nextNet, err := parseLoad(string(out), time.Now(), *cpu, *net)
	if err != nil {
		m.publishLoad(serverLoad{}, "无法读取负载")
		return m.generationAlive(gen)
	}
	*cpu = nextCPU
	*net = nextNet
	m.publishLoad(load, "")
	return m.generationAlive(gen)
}

func (m *Model) generationAlive(gen int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generation == gen
}

func (m *Model) publishLoad(load serverLoad, note string) {
	m.mu.Lock()
	m.pendingLoad = &load
	m.pendingLoadNote = note
	m.mu.Unlock()
	m.wakeUp()
}

func (m *Model) applyLoad() {
	m.mu.Lock()
	load := m.pendingLoad
	note := m.pendingLoadNote
	m.pendingLoad = nil
	m.pendingLoadNote = ""
	m.mu.Unlock()
	if load == nil && note == "" {
		return
	}
	if load != nil {
		m.load = *load
		m.loadOK = note == ""
	}
	m.loadNote = note
}

func parseLoad(text string, now time.Time, prevCPU cpuSample, prevNet netSample) (serverLoad, cpuSample, netSample, error) {
	var load serverLoad
	var total, idle, rx, tx uint64
	var sawCPU, sawNet bool
	section := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch line {
		case "LOAD", "MEM", "CPU", "NET", "UP", "DISK":
			section = line
			continue
		}
		fields := strings.Fields(line)
		switch section {
		case "LOAD":
			if len(fields) >= 3 {
				load.Load1, _ = strconv.ParseFloat(fields[0], 64)
				load.Load5, _ = strconv.ParseFloat(fields[1], 64)
				load.Load15, _ = strconv.ParseFloat(fields[2], 64)
			}
		case "MEM":
			if len(fields) < 2 {
				continue
			}
			value, _ := strconv.ParseUint(fields[1], 10, 64)
			value *= 1024
			switch fields[0] {
			case "MemTotal:":
				load.MemTotal = value
			case "MemAvailable:":
				if load.MemTotal >= value {
					load.MemUsed = load.MemTotal - value
				}
			}
		case "CPU":
			if len(fields) < 5 || fields[0] != "cpu" {
				continue
			}
			var sum uint64
			for _, field := range fields[1:] {
				n, _ := strconv.ParseUint(field, 10, 64)
				sum += n
			}
			idle, _ = strconv.ParseUint(fields[4], 10, 64)
			total = sum
			sawCPU = true
		case "NET":
			if len(fields) < 10 || strings.HasPrefix(fields[0], "Inter") || strings.HasPrefix(fields[0], "face") {
				continue
			}
			name := strings.TrimSuffix(fields[0], ":")
			if name == "lo" {
				continue
			}
			in, _ := strconv.ParseUint(fields[1], 10, 64)
			out, _ := strconv.ParseUint(fields[9], 10, 64)
			rx += in
			tx += out
			sawNet = true
		case "UP":
			if len(fields) >= 1 {
				load.Uptime, _ = strconv.ParseFloat(fields[0], 64)
			}
		case "DISK":
			if len(fields) >= 4 {
				load.DiskTotal, _ = strconv.ParseUint(fields[1], 10, 64)
				load.DiskUsed, _ = strconv.ParseUint(fields[2], 10, 64)
			}
		}
	}
	nextCPU := cpuSample{total: total, idle: idle, when: now}
	if sawCPU && prevCPU.total > 0 && total > prevCPU.total {
		dt := float64(total - prevCPU.total)
		di := float64(idle - prevCPU.idle)
		load.CPU = (1 - di/dt) * 100
		if load.CPU < 0 {
			load.CPU = 0
		}
		if load.CPU > 100 {
			load.CPU = 100
		}
	}
	nextNet := netSample{rx: rx, tx: tx, when: now}
	if sawNet && !prevNet.when.IsZero() {
		seconds := now.Sub(prevNet.when).Seconds()
		if seconds > 0 && rx >= prevNet.rx && tx >= prevNet.tx {
			load.RxRate = float64(rx-prevNet.rx) / seconds
			load.TxRate = float64(tx-prevNet.tx) / seconds
		}
	}
	load.Rx, load.Tx = rx, tx
	if load.MemTotal == 0 && load.Load1 == 0 && !sawCPU {
		return serverLoad{}, nextCPU, nextNet, fmt.Errorf("空数据")
	}
	return load, nextCPU, nextNet, nil
}

func (m *Model) paintLoad(frame *flex.Frame, ink skiffPalette) {
	frame.Column().FillWidth().Pad(12).Gap(6).Radius(12).Background(ink.elevated).Border(1, ink.line).Children(func() {
		frame.Row().Items(flex.Center).Children(func() {
			frame.Text("负载").Bold().Color(ink.ink)
			frame.Spacer()
			if m.connected && m.loadOK {
				frame.Text(fmt.Sprintf("CPU %.0f%%", m.load.CPU)).Color(ink.accent).Bold()
			}
		})
		if !m.connected {
			frame.Text("连接后显示这台服务器的负载").Color(ink.muted)
			return
		}
		if !m.loadOK {
			note := m.loadNote
			if note == "" {
				note = "正在读取"
			}
			frame.Text(note).Color(ink.muted)
			return
		}
		for _, line := range loadDetailLines(m.load) {
			frame.Text(line).Color(ink.muted)
		}
	})
}

func loadDetailLines(load serverLoad) []string {
	return []string{
		fmt.Sprintf("内存  %s / %s", formatBytes(load.MemUsed), formatBytes(load.MemTotal)),
		fmt.Sprintf("磁盘  %s / %s", formatBytes(load.DiskUsed), formatBytes(load.DiskTotal)),
		fmt.Sprintf("均值  %.2f  %.2f  %.2f", load.Load1, load.Load5, load.Load15),
		fmt.Sprintf("流量  ↑ %s/s  ↓ %s/s", formatBytes(uint64(load.TxRate)), formatBytes(uint64(load.RxRate))),
		"运行  " + formatUptime(load.Uptime),
	}
}

func formatBytes(n uint64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/(1024*1024*1024))
	}
}

func formatUptime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	d := int(seconds)
	days := d / 86400
	hours := (d % 86400) / 3600
	mins := (d % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%d天 %d小时", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时 %d分", hours, mins)
	}
	return fmt.Sprintf("%d分", mins)
}
