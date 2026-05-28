package tray

import (
	"log"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/overseer/overseer/cmd/desktop/internal/autostart"
	"github.com/overseer/overseer/cmd/desktop/internal/cache"
	"github.com/overseer/overseer/cmd/desktop/internal/config"
	"github.com/overseer/overseer/cmd/desktop/internal/locale"
	"github.com/overseer/overseer/cmd/desktop/internal/notifier"
	"github.com/overseer/overseer/cmd/desktop/internal/setup"
	"github.com/overseer/overseer/cmd/desktop/internal/wsclient"
)

// Tray represents the system tray integration.
type Tray struct {
	cfg      *config.Config
	ws       *wsclient.Client
	notifier *notifier.Notifier
	cache    *cache.Cache
}

// New creates a new Tray instance.
func New(cfg *config.Config, ws *wsclient.Client, n *notifier.Notifier, c *cache.Cache) *Tray {
	return &Tray{cfg: cfg, ws: ws, notifier: n, cache: c}
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

	// Status
	mStatus := systray.AddMenuItem(locale.T("tray.connected"), "")
	mStatus.Disable()

	systray.AddSeparator()

	// Recent notifications submenu
	mRecent := systray.AddMenuItem(locale.T("tray.recent"), "")
	var recentItems []*systray.MenuItem
	if t.cache != nil {
		entries, _ := t.cache.Recent(5)
		if len(entries) == 0 {
			item := mRecent.AddSubMenuItem(locale.T("tray.no_history"), "")
			item.Disable()
			recentItems = append(recentItems, item)
		} else {
			for _, e := range entries {
				title := e.Title
				if len(title) > 30 {
					title = title[:30] + "..."
				}
				item := mRecent.AddSubMenuItem(title, e.URL)
				recentItems = append(recentItems, item)
			}
		}
		mRecent.AddSubMenuItem("---", "")
		mViewAll := mRecent.AddSubMenuItem(locale.T("tray.view_all"), "")
		recentItems = append(recentItems, mViewAll)
	}

	systray.AddSeparator()

	// Mute
	mMute := systray.AddMenuItemCheckbox(locale.T("tray.mute"), "", false)

	// Auto-start
	mAutoStart := systray.AddMenuItemCheckbox(locale.T("tray.autostart"), "", autostart.IsEnabled())

	// Settings
	mSettings := systray.AddMenuItem(locale.T("tray.settings"), "")

	// Open Web UI
	mOpenUI := systray.AddMenuItem(locale.T("tray.open_webui"), "")

	// Reconnect
	mReconnect := systray.AddMenuItem(locale.T("tray.reconnect"), "")

	systray.AddSeparator()

	// Quit
	mQuit := systray.AddMenuItem(locale.T("tray.quit"), "")

	// Status monitor
	go func() {
		for {
			<-timeAfter2s()
			if t.ws.Connected() {
				mStatus.SetTitle(locale.T("tray.connected"))
			} else {
				mStatus.SetTitle(locale.T("tray.disconnected"))
			}
		}
	}()

	// Handle recent notification clicks
	if t.cache != nil {
		for i, item := range recentItems {
			go func(idx int, mi *systray.MenuItem) {
				for range mi.ClickedCh {
					// Last item is "View All"
					entries, _ := t.cache.Recent(5)
					if idx >= len(entries) {
						// View All
						t.openBrowser(t.cfg.ServerURL + "/#/history")
					} else if idx < len(entries) {
						url := entries[idx].URL
						if url == "" {
							url = t.cfg.ServerURL
						}
						t.openBrowser(url)
					}
				}
			}(i, item)
		}
	}

	// Menu event loop
	for {
		select {
		case <-mMute.ClickedCh:
			if mMute.Checked() {
				mMute.Uncheck()
				t.notifier.SetMuted(false)
				log.Println("[tray]", locale.T("status.unmuted"))
			} else {
				mMute.Check()
				t.notifier.SetMuted(true)
				log.Println("[tray]", locale.T("status.muted"))
			}
		case <-mAutoStart.ClickedCh:
			if mAutoStart.Checked() {
				mAutoStart.Uncheck()
				if err := autostart.Disable(); err != nil {
					log.Printf("[tray] disable autostart: %v", err)
				}
			} else {
				mAutoStart.Check()
				if err := autostart.Enable(); err != nil {
					log.Printf("[tray] enable autostart: %v", err)
				}
			}
		case <-mSettings.ClickedCh:
			log.Println("[tray] opening settings...")
			go func() {
				newURL, reReg, err := setup.ShowSettingsDialog(t.cfg)
				if err != nil {
					log.Printf("[tray] settings error: %v", err)
					return
				}
				if reReg {
					// Re-register: clear credentials and show setup dialog
					t.cfg.APIKey = ""
					t.cfg.ServerURL = newURL
					_ = config.Save(t.cfg)
					log.Println("[tray] re-registration requested, please restart the app")
					return
				}
				if newURL != "" && newURL != t.cfg.ServerURL {
					t.cfg.ServerURL = newURL
					_ = config.Save(t.cfg)
					log.Printf("[tray] server URL updated to %s, reconnecting...", newURL)
					t.ws.Reconnect()
				}
			}()
		case <-mOpenUI.ClickedCh:
			t.openBrowser(t.cfg.ServerURL)
		case <-mReconnect.ClickedCh:
			log.Println("[tray] reconnecting...")
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
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.SysProcAttr = hiddenWindowAttr()
	if err := cmd.Start(); err != nil {
		log.Printf("[tray] failed to open browser: %v", err)
	}
}
