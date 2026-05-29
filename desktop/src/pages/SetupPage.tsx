import { useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { emit } from "@tauri-apps/api/event";
import { getCurrentWindow } from "@tauri-apps/api/window";

export default function SetupPage() {
  const [serverUrl, setServerUrl] = useState("http://localhost:9721");
  const [token, setToken] = useState("");
  const [deviceName, setDeviceName] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleConnect = async () => {
    if (!serverUrl || !token) {
      setError("请填写服务器地址和注册令牌");
      return;
    }

    setLoading(true);
    setError("");

    try {
      await invoke("register_device", {
        serverUrl: serverUrl.replace(/\/+$/, ""),
        token,
        deviceName,
      });

      // Notify the main process that setup is complete
      await emit("setup-complete", {});

      // Close setup window
      const win = getCurrentWindow();
      await win.close();
    } catch (e: any) {
      setError(typeof e === "string" ? e : e.message || "注册失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="setup-container">
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
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder="从服务器 config.yaml 中获取"
        />
      </div>

      <div className="form-group">
        <label>设备名称（可选）</label>
        <input
          type="text"
          value={deviceName}
          onChange={(e) => setDeviceName(e.target.value)}
          placeholder="留空则使用主机名"
        />
      </div>

      {error && <p className="error-msg">{error}</p>}

      <div className="btn-group">
        <button
          className="btn btn-primary"
          onClick={handleConnect}
          disabled={loading}
        >
          {loading ? "连接中..." : "连接"}
        </button>
      </div>
    </div>
  );
}
