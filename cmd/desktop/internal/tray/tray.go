package tray

import (
	"log"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/overseer/overseer/cmd/desktop/internal/config"
	"github.com/overseer/overseer/cmd/desktop/internal/notifier"
	"github.com/overseer/overseer/cmd/desktop/internal/wsclient"
)

// Tray represents the system tray integration.
type Tray struct {
	cfg      *config.Config
	ws       *wsclient.Client
	notifier *notifier.Notifier
}

// New creates a new Tray instance.
func New(cfg *config.Config, ws *wsclient.Client, n *notifier.Notifier) *Tray {
	return &Tray{cfg: cfg, ws: ws, notifier: n}
}

// Run starts the system tray. This blocks until Quit is called.
func (t *Tray) Run() {
	systray.Run(t.onReady, t.onExit)
}

// Quit exits the system tray.
func (t *Tray) Quit() {
	systray.Quit()
}

func (t *Tray) onReady() {
	systray.SetTitle("Overseer")
	systray.SetTooltip("Overseer - " + t.cfg.DeviceName)
	systray.SetIcon(iconData)

	// Status (non-clickable)
	mStatus := systray.AddMenuItem("● 已连接", "连接状态")
	mStatus.Disable()

	systray.AddSeparator()

	// Mute toggle
	mMute := systray.AddMenuItemCheckbox("静音", "静音通知", false)

	// Open Web UI
	mOpenUI := systray.AddMenuItem("打开控制台", "在浏览器中打开 Overseer")

	// Reconnect
	mReconnect := systray.AddMenuItem("重新连接", "强制重连 WebSocket")

	systray.AddSeparator()

	// Quit
	mQuit := systray.AddMenuItem("退出", "退出 Overseer")

	// Status monitor
	go func() {
		for {
			<-timeAfter2s()
			if t.ws.Connected() {
				mStatus.SetTitle("● 已连接")
			} else {
				mStatus.SetTitle("○ 已断开")
			}
		}
	}()

	// Menu event loop
	for {
		select {
		case <-mMute.ClickedCh:
			if mMute.Checked() {
				mMute.Uncheck()
				t.notifier.SetMuted(false)
				log.Println("[tray] 通知已取消静音")
			} else {
				mMute.Check()
				t.notifier.SetMuted(true)
				log.Println("[tray] 通知已静音")
			}
		case <-mOpenUI.ClickedCh:
			t.openBrowser(t.cfg.ServerURL)
		case <-mReconnect.ClickedCh:
			log.Println("[tray] 正在重新连接...")
			t.ws.Reconnect()
		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (t *Tray) onExit() {
	log.Println("[tray] exiting")
}

func (t *Tray) openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[tray] failed to open browser: %v", err)
	}
}
