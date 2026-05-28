# Overseer Desktop Client Design

## Overview

轻量级桌面通知接收客户端，运行在系统托盘中，通过 WebSocket 实时接收 Overseer 后端推送的通知，并显示为操作系统原生通知。高级功能（历史记录、设置、频道管理）通过点击跳转到 Web UI 完成。

## Goals

- 纯 Go 实现，无 WebView，资源占用极低
- 跨平台：Windows（主要）、macOS、Linux
- 首次启动自动注册设备
- WebSocket 断线自动重连（指数退避）
- 原生系统通知，点击可打开 Web UI
- 系统托盘常驻，支持静音

## Non-Goals

- 不内置消息历史查看器（跳转 Web UI）
- 不在桌面端管理频道/设置
- 除托盘菜单外无独立 UI 窗口

---

## Section 1: Backend Changes

### 1.1 Database Migration (`006_desktop_devices.go`)

```sql
ALTER TABLE devices ADD COLUMN type TEXT NOT NULL DEFAULT 'bark';
CREATE INDEX IF NOT EXISTS idx_devices_type_key ON devices(type, device_key);
```

- 迁移幂等（先检查列是否存在再 ALTER）
- 现有 Bark 设备自动获得 `type = 'bark'`

### 1.2 Device Model

```go
type Device struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    DeviceKey string    `json:"device_key"`
    Type      string    `json:"type"`       // "bark" | "desktop"
    IsDefault bool      `json:"is_default"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### 1.3 Config (`config.yaml`)

```yaml
desktop:
  register_token: "your-secret-token"  # 留空则禁用注册
  max_devices: 5                        # 0 = 不限制
```

### 1.4 Desktop 自注册 API

`POST /api/devices/desktop-register`（无需 JWT，但需要 token）

**请求：**
```json
{
  "name": "My PC",
  "token": "your-secret-token"
}
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "uuid",
    "name": "My PC",
    "device_key": "dk_4ce4abf8c415ad69c3a49010dc83f359",
    "type": "desktop",
    "is_default": false
  }
}
```

安全机制：
- `token` 必须匹配 `config.yaml` 中的 `desktop.register_token`
- `max_devices` 限制最大注册数量
- `register_token` 为空时返回 403 禁止注册

### 1.5 WebSocket 认证扩展

`/ws` 端点新增 `api_key` 查询参数认证：

1. 如果 `?api_key=xxx` 存在 → 查 `devices` 表 `WHERE device_key = ? AND type = 'desktop'`
2. 找到 → 允许连接
3. 未找到 → 401
4. 无 `api_key` 参数 → 走原有 JWT/Cookie 认证（Web UI 兼容）

### 1.6 Bark 推送可选

当数据库中没有 `type = 'bark'` 的设备时，跳过 Bark 推送，不报错。WebSocket 广播始终执行，确保 desktop 客户端能收到通知。

---

## Section 2: Desktop Client 架构

目录：`cmd/desktop/`

### 2.1 包结构

```
cmd/desktop/
├── main.go                          # 入口：参数解析、注册、启动
├── internal/
│   ├── config/config.go             # 本地 JSON 配置读写
│   ├── api/api.go                   # HTTP 客户端（注册）
│   ├── setup/
│   │   ├── setup.go                 # 首次配置逻辑
│   │   ├── dialog_windows.go       # Windows WPF 配置对话框
│   │   └── dialog_other.go         # macOS/Linux 终端交互
│   ├── wsclient/wsclient.go        # WebSocket 客户端（重连、事件解析）
│   ├── notifier/
│   │   ├── notifier.go             # 通用接口 + 静音控制
│   │   ├── notifier_windows.go     # Windows Toast (go-toast/toast)
│   │   ├── notifier_darwin.go      # macOS osascript
│   │   └── notifier_linux.go       # Linux notify-send
│   └── tray/
│       ├── tray.go                  # systray 集成 + 菜单
│       ├── icon.go                  # 程序化生成 ICO 图标
│       └── timer.go                 # 定时器辅助
```

### 2.2 config 包

本地配置文件路径：
- Windows: `%LOCALAPPDATA%/overseer-desktop/config.json`
- macOS/Linux: `~/.config/overseer-desktop/config.json`

内容：
```json
{
  "server_url": "http://localhost:9721",
  "api_key": "dk_4ce4abf8c415ad69c3a49010dc83f359",
  "device_id": "uuid",
  "device_name": "Desktop HOSTNAME"
}
```

### 2.3 wsclient 包

- 连接地址：`ws(s)://server/ws?api_key=xxx`
- 断线重连：指数退避 1s → 2s → 4s → 8s → ... → 60s（上限）
- 连接成功后重置退避
- 解析 `{"type":"push","payload":{...}}` 事件，调用 notifier

### 2.4 notifier 包

**通用功能：**
- `SetMuted(bool)` / `IsMuted()` — 静音控制
- 静音时只记日志，不弹通知

**Windows 实现 (`go-toast/toast`)：**
- AppID: `"Overseer"` — Action Center 中显示的应用名
- 首次运行自动创建 Start Menu 快捷方式（注册 AppUserModelID）
- 首次运行自动生成 64x64 蓝色圆形图标（PowerShell + System.Drawing）
- 点击通知 → 打开 Web UI URL
- 声音：`toast.Default`

**macOS 实现：**
- `osascript -e 'display notification ...'`
- 零 CGO 依赖

**Linux 实现：**
- `notify-send title body`

### 2.5 tray 包 (`github.com/getlantern/systray`)

系统托盘菜单（中文）：
```
● 已连接          （状态，不可点击）
─────────────────
☐ 静音            （勾选后静音通知）
  打开控制台       （浏览器打开 Web UI）
  重新连接         （强制 WebSocket 重连）
─────────────────
  退出
```

图标：程序化生成的 16x16 蓝色圆形 ICO（内嵌 PNG → ICO 转换）

---

## Section 3: 启动与数据流

```
overseer-desktop --server http://localhost:9721 --token SECRET
  │
  ▼
读取 config.json
  │
  ├── api_key 为空？─────────────────────┐
  │                                       │
  │                          POST /api/devices/desktop-register
  │                                       │
  │                          保存 api_key + device_id 到 config.json
  │                                       │
  ├───────────────────────────────────────┘
  │
  ▼
启动 WebSocket 连接 (ws://server/ws?api_key=dk_xxx)
  │
  ▼
启动系统托盘（阻塞主线程）
  │
  ▼
后端收到推送请求
  │
  ▼
Pipeline 处理消息：
  ├── 有 Bark 设备 → Bark 推送
  └── 无 Bark 设备 → 跳过（不报错）
  │
  ▼
Hub.Broadcast({"type":"push","payload":{id,title,body,url,...}})
  │
  ▼
所有 WebSocket 客户端收到事件
  │
  ▼
Desktop wsclient 解析事件
  │
  ├── 已静音 → 仅记日志
  └── 未静音 → notifier.Show(title, body, url)
                    │
                    ▼
              Windows Toast 通知弹出
              （右下角，进入 Action Center）
                    │
                    ▼
              用户点击 → 浏览器打开 URL
```

---

## Section 4: 错误处理

| 场景 | 行为 |
|------|------|
| WebSocket 断开 | 指数退避重连，托盘显示"○ 已断开" |
| 注册失败（后端不可达） | 打印错误并退出，提示用户检查 --server 参数 |
| 通知显示失败 | 仅记日志，不中断运行 |
| 配置文件损坏/缺失 | 视为首次启动，进入注册流程 |
| api_key 无效（WS 401） | 重连时会持续失败，日志提示；未来可自动清除并重注册 |
| 无 Bark 设备 | 后端跳过 Bark 推送，标记为成功，WebSocket 广播正常 |

---

## Section 5: 使用方式

### 用户下载安装（开箱即用）

1. 从 GitHub Releases 下载对应平台的可执行文件
2. 双击运行
3. 首次启动弹出配置窗口：
   - 输入 Overseer 服务器地址（如 `http://your-server:9721`）
   - 输入注册令牌（从服务器 config.yaml 中获取）
   - 可选：自定义设备名称
4. 点击"连接"，自动注册并开始接收通知
5. 后续启动直接进入托盘模式，无需再次配置

### 高级用法（命令行）

```bash
# 通过命令行参数注册（适合脚本/自动化）
overseer-desktop --server http://localhost:9721 --token your-token --name "我的电脑"

# 查看版本
overseer-desktop --version
```

### 后端配置

在 `config.yaml` 中启用 desktop 注册：

```yaml
desktop:
  register_token: "your-secret-token"
  max_devices: 5
```

---

## Section 6: CI/CD 自动发布

### GitHub Actions（`.github/workflows/release-desktop.yml`）

打 tag 触发自动构建和发布：

```bash
git tag desktop-v1.0.0
git push origin desktop-v1.0.0
```

自动构建以下平台：
- `overseer-desktop-windows-amd64.exe`
- `overseer-desktop-windows-arm64.exe`
- `overseer-desktop-macos-amd64`
- `overseer-desktop-macos-arm64`
- `overseer-desktop-linux-amd64`

构建完成后自动创建 GitHub Release 并附带所有二进制文件。

---

## Section 7: 依赖

| 依赖 | 用途 | 平台 |
|------|------|------|
| `github.com/gorilla/websocket` | WebSocket 客户端 | 全平台 |
| `github.com/getlantern/systray` | 系统托盘 | 全平台 |
| `github.com/go-toast/toast` | Windows Toast 通知 | Windows |
| `github.com/google/uuid` | 设备 ID 生成 | 后端 |

---

## Section 8: 未来改进

- [ ] 通知支持图片/图标（从消息 payload 中获取）
- [ ] 通知支持不同声音（映射 Bark 的 sound 字段）
- [ ] Web UI 设备列表显示 desktop 设备在线/离线状态
- [ ] api_key 失效后自动重注册
- [ ] 开机自启动（Windows 注册表 / macOS LaunchAgent）
- [ ] 自定义通知弹窗位置（需要自绘 UI，脱离系统 Toast）
- [ ] 多语言支持（当前托盘菜单为中文）
