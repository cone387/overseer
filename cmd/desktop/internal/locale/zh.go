package locale

var zhStrings = map[string]string{
	// Tray menu
	"tray.connected":    "● 已连接",
	"tray.disconnected": "○ 已断开",
	"tray.mute":         "静音",
	"tray.autostart":    "开机自启",
	"tray.settings":     "设置",
	"tray.open_webui":   "打开控制台",
	"tray.reconnect":    "重新连接",
	"tray.quit":         "退出",
	"tray.recent":       "最近通知",
	"tray.view_all":     "查看全部",
	"tray.no_history":   "暂无通知",

	// Setup dialog
	"setup.title":        "Overseer Desktop - 首次配置",
	"setup.welcome":      "欢迎使用 Overseer Desktop",
	"setup.server_url":   "服务器地址",
	"setup.token":        "注册令牌",
	"setup.device_name":  "设备名称（可选）",
	"setup.connect":      "连接",
	"setup.cancel":       "取消",
	"setup.validation":   "请填写服务器地址和注册令牌",

	// Settings dialog
	"settings.title":       "Overseer Desktop - 设置",
	"settings.server_url":  "服务器地址",
	"settings.device_name": "设备名称",
	"settings.device_id":   "设备 ID",
	"settings.re_register": "重新注册",
	"settings.reset":       "重置配置",
	"settings.save":        "保存",
	"settings.cancel":      "取消",

	// Notifications
	"notify.update_title":   "发现新版本",
	"notify.update_message": "Overseer Desktop %s 已发布，点击下载更新",
	"notify.open":           "打开",

	// Status
	"status.muted":   "通知已静音",
	"status.unmuted": "通知已取消静音",
}
