# Skiff

一艘只做 SSH 的小艇。用 [Viva](https://github.com/vivaui/viva) 画原生窗口，SSH 会话、终端网格和主机数据放在本仓库的 Go 代码里。

第一个能用的版本只做四件事：

1. 保存主机（地址、用户、端口）。
2. 连接前校验主机密钥，通过后再做认证。
3. 一个等宽终端网格，能输入、显示输出，并跟随窗口改 PTY 大小。
4. 断开时关掉 SSH 通道和 PTY，不留后台会话。

不做 SFTP、端口转发、VNC/RDP、AI、跳板、分屏，也不导入其他客户端的配置。

## 分层

| 包 | 职责 | 不放什么 |
| --- | --- | --- |
| `session` | `ssh.Client`、认证、known_hosts、PTY 请求 | 任何 Viva 类型 |
| `term` | 解析终端输出，维护字符单元格和光标 | 网络连接 |
| `ui` | 主机列表、标签、把单元格画进窗口 | SSH 握手细节 |
| `store` | 主机清单；口令走系统密钥库 | 自己发明的加密文件格式 |

界面回调里只发「按键、改尺寸、请求连接」。读 SSH 的循环放在独立 goroutine，把新的一帧单元格交给 UI。网格没变化就不刷新。

主机密钥不匹配就停止，不继续发密码或私钥。口令使用 Viva 已有的 `app.StoreSecret`、`LoadSecret`、`DeleteSecret`（Windows 上是 Credential Manager）。

## 环境

需要 Go 1.26 或更新。Windows 上还需要 Win32、Direct3D 11 和 DirectWrite。Viva 的 Windows/D3D11 后端是实验性的，先在本机跑通 `viva doctor`。

```powershell
scoop bucket add vivaui https://github.com/vivaui/homebrew-tap
scoop install viva
viva doctor
```

依赖固定到已发布版本，不要使用 `replace => ../viva`。界面用 `viva dev .` 看热重载；`session` 等包用普通 `go test`，不需要开窗口。

## 实现顺序

1. 只画主机列表和一个静态等宽文本区，确认窗口、焦点和中文输入可用。
2. 接本地 shell（`github.com/creack/pty`），打通键盘、输出和 resize。这一步不过网。
3. 换成 `ssh.Session` 请求远端 PTY。认证先支持密钥和密码。
4. 持久化主机和 known_hosts。未知指纹必须显式确认。
5. 稳定后再考虑 SFTP。若以后做端口转发，只监听 loopback，并随会话关闭。

计划中的依赖：界面 `github.com/vivaui/viva`，SSH `golang.org/x/crypto`，本地 PTY `github.com/creack/pty`。第一版不引入现成的终端模拟器库，VT 网格由 `term` 自己维护。Viva 提供 `ui.MonoFamily` 画等宽字，没有现成的 SSH 终端组件。
