import { useEffect, useState } from "react";
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

function App() {
  const [page, setPage] = useState<"loading" | "setup" | "main">("loading");
  const [config, setConfig] = useState<Config | null>(null);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [wsStatus, setWsStatus] = useState("disconnected");
  const [error, setError] = useState("");

  // Setup form state
  const [serverUrl, setServerUrl] = useState("http://localhost:9721");
  const [token, setToken] = useState("");
  const [deviceName, setDeviceName] = useState("");
  const [registering, setRegistering] = useState(false);

  useEffect(() => {
    // Check if already configured
    invoke<Config>("get_config").then((cfg) => {
      setConfig(cfg);
      if (cfg.server_url && cfg.api_key) {
        setPage("main");
        loadNotifications();
      } else {
        setPage("setup");
      }
    });

    // Listen for events from Rust backend
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
      const items = await invoke<Notification[]>("get_recent_notifications", { count: 20 });
      setNotifications(items);
      const count = await invoke<number>("get_unread_count");
      setUnreadCount(count);
    } catch (e) {
      console.error("load notifications:", e);
    }
  };

  const handleRegister = async () => {
    if (!serverUrl || !token) {
      setError("请填写服务器地址和注册令牌");
      return;
    }
    setError("");
    setRegistering(true);
    try {
      const name = await invoke<string>("register_device", {
        serverUrl,
        token,
        deviceName,
      });
      // Reload config
      const cfg = await invoke<Config>("get_config");
      setConfig(cfg);
      setPage("main");
      // Restart needed to connect WS - for now just show success
      setWsStatus("connected");
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

  // ─── Setup Page ───────────────────────────────────────────────────────────
  if (page === "loading") {
    return (
      <main className="container">
        <p>加载中...</p>
      </main>
    );
  }

  if (page === "setup") {
    return (
      <main className="container">
        <h1>Overseer Desktop</h1>
        <p className="subtitle">首次配置 - 连接到 Overseer 服务器</p>

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
      </main>
    );
  }

  // ─── Main Page ────────────────────────────────────────────────────────────
  return (
    <main className="container">
      <div className="header">
        <h1>Overseer Desktop</h1>
        <div className="status">
          <span className={`dot ${wsStatus === "connected" ? "green" : "red"}`} />
          <span>{wsStatus === "connected" ? "已连接" : "已断开"}</span>
          {config && <span className="device-name">{config.device_name}</span>}
        </div>
      </div>

      <div className="toolbar">
        <span className="unread-badge">
          {unreadCount > 0 ? `${unreadCount} 条未读` : "无未读"}
        </span>
        {unreadCount > 0 && (
          <button className="btn-small" onClick={handleMarkAllRead}>
            全部已读
          </button>
        )}
      </div>

      <div className="notification-list">
        {notifications.length === 0 ? (
          <p className="empty">暂无通知</p>
        ) : (
          notifications.map((n) => (
            <div
              key={n.id}
              className={`notification-item ${n.unread ? "unread" : ""}`}
            >
              <div className="notification-header">
                <span className="channel">[{n.channel || "default"}]</span>
                <span className="time">
                  {new Date(n.received_at).toLocaleString()}
                </span>
              </div>
              <div className="notification-title">{n.title}</div>
              {n.body && <div className="notification-body">{n.body}</div>}
            </div>
          ))
        )}
      </div>
    </main>
  );
}

export default App;
