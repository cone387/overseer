import { useEffect, useState, useMemo } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
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

type Page = "loading" | "setup" | "main" | "settings";

function App() {
  const [page, setPage] = useState<Page>("loading");
  const [config, setConfig] = useState<Config | null>(null);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [wsStatus, setWsStatus] = useState("disconnected");
  const [error, setError] = useState("");
  const [autostart, setAutostart] = useState(false);
  const [activeChannel, setActiveChannel] = useState<string>("all");

  // Setup form state
  const [serverUrl, setServerUrl] = useState("http://localhost:9721");
  const [token, setToken] = useState("");
  const [deviceName, setDeviceName] = useState("");
  const [registering, setRegistering] = useState(false);

  useEffect(() => {
    invoke<Config>("get_config").then((cfg) => {
      setConfig(cfg);
      if (cfg.server_url && cfg.api_key) {
        setPage("main");
        loadNotifications();
      } else {
        setPage("setup");
      }
    });

    invoke<boolean>("get_autostart_enabled").then(setAutostart);

    const unlisten1 = listen<string>("ws-status", (event) => {
      setWsStatus(event.payload);
      if (event.payload === "auth_failed") {
        setPage("setup");
        setError("认证失败，请重新注册");
      }
    });

    const unlisten2 = listen<number>("unread-count", (event) => {
      setUnreadCount(event.payload);
      loadNotifications();
    });

    return () => {
      unlisten1.then((f) => f());
      unlisten2.then((f) => f());
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

  // Group notifications by channel
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

  const handleRegister = async () => {
    if (!serverUrl || !token) {
      setError("请填写服务器地址和注册令牌");
      return;
    }
    setError("");
    setRegistering(true);
    try {
      await invoke<string>("register_device", { serverUrl, token, deviceName });
      const cfg = await invoke<Config>("get_config");
      setConfig(cfg);
      setPage("main");
      setWsStatus("connected");
      loadNotifications();
    } catch (e: any) {
      setError(typeof e === "string" ? e : e.message || "注册失败");
    } finally {
      setRegistering(false);
    }
  };

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

  // ─── Loading ──────────────────────────────────────────────────────────────
  if (page === "loading") {
    return <main className="container center"><p>加载中...</p></main>;
  }

  // ─── Setup Page ───────────────────────────────────────────────────────────
  if (page === "setup") {
    return (
      <main className="container center">
        <div className="setup-card">
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
            <label>注册令牌</label>
            <input
              type="text"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="从服务器配置中获取"
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

          <button onClick={handleRegister} disabled={registering}>
            {registering ? "连接中..." : "连接"}
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
          <button className="btn-back" onClick={() => setPage("main")}>←</button>
          <h2>设置</h2>
        </div>

        <div className="settings-section">
          <div className="setting-item">
            <span className="setting-label">服务器地址</span>
            <span className="setting-value">{config?.server_url}</span>
          </div>
          <div className="setting-item">
            <span className="setting-label">设备名称</span>
            <span className="setting-value">{config?.device_name}</span>
          </div>
          <div className="setting-item">
            <span className="setting-label">设备 ID</span>
            <span className="setting-value mono">{config?.device_id}</span>
          </div>
          <div className="setting-item">
            <span className="setting-label">开机自启</span>
            <label className="toggle">
              <input type="checkbox" checked={autostart} onChange={handleToggleAutostart} />
              <span className="toggle-slider" />
            </label>
          </div>
        </div>

        <div className="settings-actions">
          <button className="btn-danger" onClick={() => { setPage("setup"); setServerUrl(config?.server_url || ""); }}>
            重新注册
          </button>
        </div>
      </main>
    );
  }

  // ─── Main Page (Two-column layout) ────────────────────────────────────────
  return (
    <main className="app-layout">
      {/* Left sidebar - channel groups */}
      <aside className="sidebar">
        <div className="sidebar-header">
          <div className="app-title">Overseer</div>
          <div className={`conn-dot ${wsStatus === "connected" ? "green" : "red"}`} />
        </div>

        <nav className="channel-list">
          <div
            className={`channel-item ${activeChannel === "all" ? "active" : ""}`}
            onClick={() => setActiveChannel("all")}
          >
            <span className="channel-name">全部</span>
            {unreadCount > 0 && <span className="channel-badge">{unreadCount}</span>}
          </div>

          {Object.entries(channels).map(([ch, info]) => (
            <div
              key={ch}
              className={`channel-item ${activeChannel === ch ? "active" : ""}`}
              onClick={() => setActiveChannel(ch)}
            >
              <span className="channel-name">{ch}</span>
              {info.unread > 0 && <span className="channel-badge">{info.unread}</span>}
            </div>
          ))}
        </nav>

        <div className="sidebar-footer">
          <button className="btn-icon" onClick={() => setPage("settings")} title="设置">⚙️</button>
          <span className="device-label">{config?.device_name}</span>
        </div>
      </aside>

      {/* Right panel - notification list */}
      <section className="main-panel">
        <div className="panel-header">
          <h2>{activeChannel === "all" ? "全部通知" : activeChannel}</h2>
          <div className="panel-actions">
            {unreadCount > 0 && (
              <button className="btn-small" onClick={handleMarkAllRead}>全部已读</button>
            )}
          </div>
        </div>

        <div className="notification-list">
          {filteredNotifications.length === 0 ? (
            <div className="empty">
              <p>暂无通知</p>
            </div>
          ) : (
            filteredNotifications.map((n) => (
              <div
                key={n.id}
                className={`notification-item ${n.unread ? "unread" : ""}`}
                onClick={() => n.unread && handleAck(n.id)}
              >
                <div className="notification-meta">
                  <span className="notification-source">{n.source || n.channel || "default"}</span>
                  <span className="notification-time">
                    {formatTime(n.received_at)}
                  </span>
                </div>
                <div className="notification-title">{n.title}</div>
                {n.body && <div className="notification-body">{n.body}</div>}
              </div>
            ))
          )}
        </div>
      </section>
    </main>
  );
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const diff = now.getTime() - d.getTime();

  if (diff < 60000) return "刚刚";
  if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`;
  if (d.toDateString() === new Date(now.getTime() - 86400000).toDateString()) return "昨天";
  return d.toLocaleDateString();
}

export default App;
