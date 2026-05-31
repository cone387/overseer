import { useEffect, useState, useMemo, useRef } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { openUrl } from "@tauri-apps/plugin-opener";
import "./App.css";

interface Config {
  server_url: string;
  api_key: string;
  device_id: string;
  device_name: string;
}

interface Notification {
  id: string;
  title: string;
  body: string;
  url: string;
  channel: string;
  source: string;
  received_at: string;
  unread: boolean;
}

type Page = "loading" | "setup" | "linking" | "main" | "settings";

function App() {
  const [page, setPage] = useState<Page>("loading");
  const [config, setConfig] = useState<Config | null>(null);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [wsStatus, setWsStatus] = useState("disconnected");
  const [error, setError] = useState("");
  const [autostart, setAutostart] = useState(false);
  const [activeChannel, setActiveChannel] = useState<string>("all");

  // Setup form
  const [serverUrl, setServerUrl] = useState("http://localhost:9721");
  const [deviceName, setDeviceName] = useState("");

  // Linking state
  const [linkCode, setLinkCode] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Settings form
  const [editingUrl, setEditingUrl] = useState("");
  const [savingUrl, setSavingUrl] = useState(false);

  useEffect(() => {
    invoke<Config>("get_config").then((cfg) => {
      setConfig(cfg);
      if (cfg.server_url && cfg.api_key) {
        setPage("main");
        loadNotifications();
        invoke<boolean>("is_connected").then((connected) => {
          setWsStatus(connected ? "connected" : "disconnected");
        });
      } else {
        if (cfg.server_url) setServerUrl(cfg.server_url);
        setPage("setup");
      }
    });

    invoke<boolean>("get_autostart_enabled").then(setAutostart);

    const unlisten1 = listen<string>("ws-status", (event) => {
      setWsStatus(event.payload);
      if (event.payload === "auth_failed") {
        setPage("setup");
        setError("认证失败，请重新授权");
      }
    });

    const unlisten2 = listen<number>("unread-count", (event) => {
      setUnreadCount(event.payload);
      loadNotifications();
    });

    return () => {
      unlisten1.then((f) => f());
      unlisten2.then((f) => f());
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  const loadNotifications = async () => {
    try {
      const items = await invoke<Notification[]>("get_recent_notifications", { count: 50 });
      setNotifications(items);
      const count = await invoke<number>("get_unread_count");
      setUnreadCount(count);
    } catch (e) {
      console.error("load notifications:", e);
    }
  };

  const channels = useMemo(() => {
    const map: Record<string, { count: number; unread: number }> = {};
    for (const n of notifications) {
      const ch = n.channel || "default";
      if (!map[ch]) map[ch] = { count: 0, unread: 0 };
      map[ch].count++;
      if (n.unread) map[ch].unread++;
    }
    return map;
  }, [notifications]);

  const filteredNotifications = useMemo(() => {
    if (activeChannel === "all") return notifications;
    return notifications.filter((n) => (n.channel || "default") === activeChannel);
  }, [notifications, activeChannel]);

  // ─── Auth Flow ────────────────────────────────────────────────────────────

  const handleStartLink = async () => {
    if (!serverUrl) {
      setError("请填写服务器地址");
      return;
    }
    setError("");
    try {
      const code = await invoke<string>("start_desktop_link", {
        serverUrl,
        deviceName,
      });
      setLinkCode(code);
      setPage("linking");

      // Open browser to the server's desktop-link page
      await openUrl(`${serverUrl}/desktop-link?code=${code}`);

      // Start polling
      startPolling(code);
    } catch (e: any) {
      setError(typeof e === "string" ? e : e.message || "连接失败");
    }
  };

  const startPolling = (code: string) => {
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = setInterval(async () => {
      try {
        const confirmed = await invoke<boolean>("poll_desktop_link", { code });
        if (confirmed) {
          if (pollRef.current) clearInterval(pollRef.current);
          const cfg = await invoke<Config>("get_config");
          setConfig(cfg);
          setPage("main");
          setWsStatus("connected");
          loadNotifications();
        }
      } catch (e: any) {
        // Code expired or error
        if (pollRef.current) clearInterval(pollRef.current);
        setError(typeof e === "string" ? e : "授权超时，请重试");
        setPage("setup");
      }
    }, 2000);
  };

  const handleCancelLink = () => {
    if (pollRef.current) clearInterval(pollRef.current);
    setPage("setup");
    setLinkCode("");
  };

  // ─── Actions ──────────────────────────────────────────────────────────────

  const handleMarkAllRead = async () => {
    await invoke("mark_all_read");
    setUnreadCount(0);
    loadNotifications();
  };

  const handleAck = async (id: string) => {
    try {
      await invoke("ack_message", { messageId: id });
      loadNotifications();
    } catch (e) {
      console.error("ack failed:", e);
    }
  };

  const handleToggleAutostart = async () => {
    const newVal = !autostart;
    try {
      await invoke("set_autostart", { enabled: newVal });
      setAutostart(newVal);
    } catch (e) {
      console.error("autostart toggle failed:", e);
    }
  };

  const handleSaveUrl = async () => {
    if (!editingUrl) return;
    setSavingUrl(true);
    try {
      await invoke("update_server_url", { serverUrl: editingUrl });
      const cfg = await invoke<Config>("get_config");
      setConfig(cfg);
      setWsStatus("connected");
    } catch (e: any) {
      console.error("update url failed:", e);
    } finally {
      setSavingUrl(false);
    }
  };

  // ─── Loading ──────────────────────────────────────────────────────────────
  if (page === "loading") {
    return <main className="container center"><div className="loading-spinner" /></main>;
  }

  // ─── Setup Page ───────────────────────────────────────────────────────────
  if (page === "setup") {
    return (
      <main className="container center">
        <div className="setup-card">
          <div className="setup-icon">📡</div>
          <h1>Overseer Desktop</h1>
          <p className="subtitle">连接到 Overseer 服务器</p>

          <div className="form-group">
            <label>服务器地址</label>
            <input
              type="text"
              value={serverUrl}
              onChange={(e) => setServerUrl(e.target.value)}
              placeholder="http://localhost:9721"
            />
          </div>

          <div className="form-group">
            <label>设备名称（可选）</label>
            <input
              type="text"
              value={deviceName}
              onChange={(e) => setDeviceName(e.target.value)}
              placeholder="留空使用默认名称"
            />
          </div>

          {error && <p className="error">{error}</p>}

          <button className="btn-primary" onClick={handleStartLink}>
            授权登录
          </button>
          <p className="setup-hint">点击后将打开浏览器完成登录授权</p>
        </div>
      </main>
    );
  }

  // ─── Linking (Waiting for confirmation) ───────────────────────────────────
  if (page === "linking") {
    return (
      <main className="container center">
        <div className="setup-card">
          <div className="linking-spinner" />
          <h2>等待授权</h2>
          <p className="subtitle">请在浏览器中完成登录并确认授权</p>

          <div className="link-code-display">
            <span className="link-code-label">授权码</span>
            <span className="link-code">{linkCode}</span>
          </div>

          <p className="linking-hint">
            如果浏览器未自动打开，请手动访问：
          </p>
          <p className="linking-url">{serverUrl}/desktop-link?code={linkCode}</p>

          <button className="btn-secondary" onClick={handleCancelLink}>
            取消
          </button>
        </div>
      </main>
    );
  }

  // ─── Settings Page ────────────────────────────────────────────────────────
  if (page === "settings") {
    return (
      <main className="container">
        <div className="page-header">
          <button className="btn-back" onClick={() => setPage("main")}><span>←</span></button>
          <h2>设置</h2>
        </div>

        <div className="settings-card">
          <div className="setting-group-title">连接</div>
          <div className="setting-item">
            <div className="setting-left">
              <span className="setting-label">服务器地址</span>
              <span className="setting-desc">WebSocket 和 API 连接地址</span>
            </div>
            <div className="setting-right">
              <input
                className="setting-input"
                type="text"
                value={editingUrl}
                onChange={(e) => setEditingUrl(e.target.value)}
                placeholder={config?.server_url}
              />
              <button
                className="btn-sm"
                onClick={handleSaveUrl}
                disabled={savingUrl || !editingUrl || editingUrl === config?.server_url}
              >
                保存
              </button>
            </div>
          </div>

          <div className="setting-group-title">设备</div>
          <div className="setting-item">
            <span className="setting-label">设备名称</span>
            <span className="setting-value">{config?.device_name}</span>
          </div>
          <div className="setting-item">
            <span className="setting-label">设备 ID</span>
            <span className="setting-value mono">{config?.device_id?.slice(0, 8)}...</span>
          </div>

          <div className="setting-group-title">通用</div>
          <div className="setting-item">
            <div className="setting-left">
              <span className="setting-label">开机自启</span>
              <span className="setting-desc">登录时自动启动 Overseer Desktop</span>
            </div>
            <label className="toggle">
              <input type="checkbox" checked={autostart} onChange={handleToggleAutostart} />
              <span className="toggle-slider" />
            </label>
          </div>
        </div>

        <div className="settings-footer">
          <button className="btn-danger" onClick={() => { setPage("setup"); setServerUrl(config?.server_url || ""); }}>
            重新授权
          </button>
        </div>
      </main>
    );
  }

  // ─── Main Page ────────────────────────────────────────────────────────────
  return (
    <main className="app-layout">
      <aside className="sidebar">
        <div className="sidebar-header">
          <div className="app-brand">
            <span className="app-logo">◉</span>
            <span className="app-title">Overseer</span>
          </div>
          <div className={`status-indicator ${wsStatus === "connected" ? "online" : "offline"}`}>
            <span className="status-dot" />
            <span className="status-text">{wsStatus === "connected" ? "在线" : "离线"}</span>
          </div>
        </div>

        <nav className="channel-list">
          <div className="channel-section-title">频道</div>
          <div
            className={`channel-item ${activeChannel === "all" ? "active" : ""}`}
            onClick={() => setActiveChannel("all")}
          >
            <span className="channel-icon">📋</span>
            <span className="channel-name">全部消息</span>
            {unreadCount > 0 && <span className="channel-badge">{unreadCount}</span>}
          </div>

          {Object.entries(channels).map(([ch, info]) => (
            <div
              key={ch}
              className={`channel-item ${activeChannel === ch ? "active" : ""}`}
              onClick={() => setActiveChannel(ch)}
            >
              <span className="channel-icon">{getChannelIcon(ch)}</span>
              <span className="channel-name">{ch}</span>
              {info.unread > 0 && <span className="channel-badge">{info.unread}</span>}
            </div>
          ))}
        </nav>

        <div className="sidebar-footer">
          <div className="user-info">
            <span className="user-avatar">👤</span>
            <span className="user-name">{config?.device_name}</span>
          </div>
          <button className="btn-icon" onClick={() => { setPage("settings"); setEditingUrl(config?.server_url || ""); }} title="设置">⚙</button>
        </div>
      </aside>

      <section className="main-panel">
        <div className="panel-header">
          <div className="panel-title">
            <h2>{activeChannel === "all" ? "全部消息" : activeChannel}</h2>
            <span className="panel-count">{filteredNotifications.length} 条</span>
          </div>
          <div className="panel-actions">
            {unreadCount > 0 && (
              <button className="btn-sm" onClick={handleMarkAllRead}>全部已读</button>
            )}
          </div>
        </div>

        <div className="notification-list">
          {filteredNotifications.length === 0 ? (
            <div className="empty-state">
              <div className="empty-icon">📭</div>
              <p>暂无通知</p>
              <span>新消息将在这里显示</span>
            </div>
          ) : (
            filteredNotifications.map((n) => (
              <div
                key={n.id}
                className={`notification-card ${n.unread ? "unread" : ""}`}
                onClick={() => n.unread && handleAck(n.id)}
              >
                <div className="notification-indicator" />
                <div className="notification-content">
                  <div className="notification-meta">
                    <span className="notification-source">{n.source || n.channel || "system"}</span>
                    <span className="notification-time">{formatTime(n.received_at)}</span>
                  </div>
                  <div className="notification-title">{n.title}</div>
                  {n.body && <div className="notification-body">{n.body}</div>}
                </div>
              </div>
            ))
          )}
        </div>
      </section>
    </main>
  );
}

function getChannelIcon(channel: string): string {
  const icons: Record<string, string> = {
    urgent: "🔴", monitor: "📊", github: "🐙",
    info: "ℹ️", success: "✅", default: "💬",
  };
  return icons[channel] || "📌";
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  if (diff < 60000) return "刚刚";
  if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`;
  return d.toLocaleDateString();
}

export default App;
