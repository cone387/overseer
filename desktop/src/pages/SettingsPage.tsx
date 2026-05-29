import { useState, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";
import { getCurrentWindow } from "@tauri-apps/api/window";

interface AppConfig {
  server_url: string;
  api_key: string;
  device_id: string;
  device_name: string;
  last_update_check?: string;
  language?: string;
  muted?: boolean;
}

export default function SettingsPage() {
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [serverUrl, setServerUrl] = useState("");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      const cfg = await invoke<AppConfig>("get_config");
      setConfig(cfg);
      setServerUrl(cfg.server_url);
    } catch (e: any) {
      setError(typeof e === "string" ? e : "加载配置失败");
    }
  };

  const handleSave = async () => {
    if (!config) return;
    setError("");
    setSuccess("");

    try {
      const updated = { ...config, server_url: serverUrl };
      await invoke("save_config", { config: updated });
      setSuccess("设置已保存");

      // If server URL changed, trigger reconnect
      if (serverUrl !== config.server_url) {
        await invoke("reconnect_ws");
      }
    } catch (e: any) {
      setError(typeof e === "string" ? e : "保存失败");
    }
  };

  const handleReRegister = async () => {
    if (!config) return;
    // Clear credentials and close settings, open setup
    try {
      const cleared = { ...config, api_key: "", device_id: "" };
      await invoke("save_config", { config: cleared });
      const win = getCurrentWindow();
      await win.close();
    } catch (e: any) {
      setError(typeof e === "string" ? e : "操作失败");
    }
  };

  const handleClose = async () => {
    const win = getCurrentWindow();
    await win.close();
  };

  if (!config) {
    return <div className="settings-container">加载中...</div>;
  }

  return (
    <div className="settings-container">
      <h1>设置</h1>

      <div className="form-group">
        <label>服务器地址</label>
        <input
          type="text"
          value={serverUrl}
          onChange={(e) => setServerUrl(e.target.value)}
        />
      </div>

      <div className="info-row">
        <span className="label">设备名称</span>
        <span className="value">{config.device_name}</span>
      </div>

      <div className="info-row">
        <span className="label">设备 ID</span>
        <span className="value">
          {config.device_id ? config.device_id.slice(0, 8) + "..." : "未注册"}
        </span>
      </div>

      <div className="info-row">
        <span className="label">API Key</span>
        <span className="value">
          {config.api_key ? config.api_key.slice(0, 12) + "..." : "未注册"}
        </span>
      </div>

      {error && <p className="error-msg">{error}</p>}
      {success && <p className="success-msg">{success}</p>}

      <div className="btn-group">
        <button className="btn btn-primary" onClick={handleSave}>
          保存
        </button>
        <button className="btn btn-danger" onClick={handleReRegister}>
          重新注册
        </button>
        <button className="btn btn-secondary" onClick={handleClose}>
          关闭
        </button>
      </div>
    </div>
  );
}
