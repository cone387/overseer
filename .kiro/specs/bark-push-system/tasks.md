# Implementation Plan: Overseer (bark-push-system)

## Overview

基于 Bark 的个人定制推送系统实现计划。采用 Go + Gin 后端、SQLite 存储、React/Vite 前端（通过 go:embed 嵌入），Docker 部署。按照依赖关系从基础设施层到业务逻辑层逐步构建。

## Tasks

- [x] 1. 项目初始化与基础设施
  - [x] 1.1 初始化 Go 模块与项目目录结构
    - 创建 `go.mod`，引入核心依赖（gin, go-sqlite3, gorilla/websocket, robfig/cron, gopkg.in/yaml.v3, pgregory.net/rapid）
    - 按照设计文档创建目录结构：cmd/overseer, internal/config, internal/server, internal/router, internal/channel, internal/aggregator, internal/scheduler, internal/escalator, internal/template, internal/pusher, internal/ws, internal/store, internal/model, web/
    - _Requirements: 12.1_

  - [x] 1.2 定义核心数据模型与接口
    - 创建 `internal/model/message.go`：Message 结构体、PushStatus 常量
    - 创建 `internal/model/reminder.go`：Reminder 结构体、RepeatType 常量
    - 创建 `internal/model/stats.go`：ChannelStats、PagedResult 结构体
    - 创建 `internal/pusher/pusher.go`：Pusher 接口、PushRequest、PushResponse
    - 创建 `internal/store/store.go`：Store 接口定义（含所有方法签名）
    - _Requirements: 10.1, 13.1, 14.1_


- [x] 2. 配置加载与验证
  - [x] 2.1 实现 YAML 配置结构定义与加载
    - 创建 `internal/config/config.go`：定义 Config、ServerConfig、BarkConfig、Channel、Rule、DNDPeriod、Template、AggregatorConfig、EscalationConfig 结构体及 YAML 标签
    - 创建 `internal/config/loader.go`：实现 YAMLLoader，支持从指定路径或环境变量 `OVERSEER_CONFIG_PATH` 或默认 `config.yaml` 加载配置
    - 处理文件不存在/无权限的错误信息（包含路径和具体原因）
    - 处理 YAML 语法错误（包含行号和错误描述）
    - _Requirements: 1.1, 1.2, 1.3, 12.2_

  - [x] 2.2 实现配置字段验证
    - 创建 `internal/config/validator.go`：实现 Validate 方法
    - 验证必填字段：server.port、bark.server_url、bark.device_key、server.api_key
    - 验证字段约束：port 范围 1-65535、api_key 长度 ≥16、server_url 合法 URL、Channel 名称唯一
    - 验证 Rule 中 content 正则表达式语法有效性
    - 验证 device_key 可用性（Channel 未配置时需有全局默认）
    - _Requirements: 1.4, 1.5, 1.6, 2.2, 3.7, 9.4, 11.3, 11.4_


  - [ ]* 2.3 编写配置模块属性测试
    - **Property 1: 配置 Round-Trip 一致性**
    - **Property 2: 必填字段缺失检测**
    - **Property 3: 字段约束违反检测**
    - **Validates: Requirements 1.4, 1.5, 1.6, 1.7**

  - [x] 2.4 创建示例配置文件
    - 创建 `config.example.yaml`，包含所有配置段的完整示例
    - _Requirements: 1.5_

- [x] 3. 数据存储层
  - [x] 3.1 实现 SQLite 存储与数据库迁移
    - 创建 `internal/store/sqlite.go`：实现 Store 接口的 SQLite 版本
    - 创建 `internal/store/migrations/` 目录及初始迁移脚本
    - 实现 messages、reminders、push_results 三张表的创建与索引
    - 实现 Migrate() 方法：自动创建/升级数据库
    - 实现 Close() 方法
    - _Requirements: 10.1, 12.5, 14.9_

  - [x] 3.2 实现消息存储 CRUD 操作
    - 实现 SaveMessage：持久化消息记录
    - 实现 UpdateMessageStatus：更新推送状态、失败原因、重试次数
    - 实现 QueryMessages：支持时间范围、Channel、状态过滤，分页，时间倒序
    - 实现 GetChannelStats：按时间范围统计各 Channel 推送数据
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6_


  - [x] 3.3 实现提醒存储 CRUD 操作
    - 实现 CreateReminder、UpdateReminder、CancelReminder
    - 实现 ListReminders：支持按状态筛选
    - 实现 GetActiveReminders：获取所有活跃提醒（用于服务重启恢复）
    - 实现 UpdateNextTrigger：更新下次触发时间
    - _Requirements: 14.1, 14.5, 14.6, 14.7, 14.9_

  - [ ]* 3.4 编写存储层属性测试
    - **Property 22: 消息持久化生命周期**
    - **Property 23: 查询 API 分页与过滤**
    - **Validates: Requirements 10.1, 10.2, 10.4, 10.5**

- [x] 4. Checkpoint - 确保基础设施层测试通过
  - 确保所有测试通过，如有问题请向用户确认。

- [x] 5. 路由引擎与模板引擎
  - [x] 5.1 实现路由引擎
    - 创建 `internal/router/router.go`：实现 Router 结构体和 NewRouter 构造函数
    - 创建 `internal/router/rule.go`：实现 CompiledRule、正则编译、规则匹配逻辑
    - 实现 Route 方法：按配置顺序 first-match 语义匹配
    - 实现 source 精确匹配 + content 正则匹配（AND 逻辑）
    - 实现未匹配时回退到 "default" 频道
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_


  - [ ]* 5.2 编写路由引擎属性测试
    - **Property 4: 路由引擎 First-Match 语义**
    - **Property 5: 路由引擎 Default 回退**
    - **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6**

  - [x] 5.3 实现频道管理
    - 创建 `internal/channel/channel.go`：频道查找、默认频道逻辑
    - 实现不存在的 Channel 回退到 "default"
    - 实现未定义 "default" 频道时使用内置默认配置
    - _Requirements: 2.1, 2.7, 2.8_

  - [x] 5.4 实现模板引擎
    - 创建 `internal/template/engine.go`：实现 Engine 结构体
    - 实现 NewEngine：解析所有配置模板为 Go template
    - 实现 Render：渲染消息内容，支持 source、title、body、timestamp、extra 字段
    - 实现模板不存在时回退原始内容 + 警告日志
    - 实现渲染出错时回退原始内容 + 警告日志
    - 实现渲染结果为空时回退原始内容 + 警告日志
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6_

  - [ ]* 5.5 编写模板引擎属性测试
    - **Property 19: 模板渲染与回退**
    - **Validates: Requirements 8.2, 8.3, 8.4, 8.5, 8.6**


- [x] 6. Bark 推送客户端
  - [x] 6.1 实现 Bark 推送客户端
    - 创建 `internal/pusher/bark.go`：实现 BarkPusher 结构体
    - 实现 NewBarkPusher：初始化 HTTP 客户端（10 秒超时）
    - 实现 Push 方法：构造 POST 请求，包含 title、body、sound、group、icon、level 参数
    - 实现指数退避重试逻辑（1s、2s、4s，最多 3 次）
    - 实现多设备推送：遍历 Channel 的 device_keys，单设备失败不影响其余
    - 未配置 device_key 时使用全局默认
    - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5, 9.1, 9.2, 9.3, 9.5, 2.3, 2.4, 2.5, 2.6_

  - [ ]* 6.2 编写 Bark 推送属性测试
    - **Property 6: Channel 配置到 Bark 推送参数映射**
    - **Property 20: 多设备独立推送**
    - **Property 21: 指数退避重试**
    - **Validates: Requirements 2.3, 2.4, 2.5, 2.6, 9.2, 9.3, 13.1, 13.2, 13.3**

- [x] 7. 消息聚合器
  - [x] 7.1 实现消息去重
    - 创建 `internal/aggregator/dedup.go`：基于 title+body 哈希的去重逻辑
    - 在配置时间窗口内相同内容仅推送第一条，附带重复计数
    - _Requirements: 4.1_

  - [x] 7.2 实现消息批量合并
    - 创建 `internal/aggregator/batch.go`：超过阈值时合并为摘要推送
    - 摘要包含消息总数、时间窗口起止时间和每条消息 title 列表
    - _Requirements: 4.2_


  - [x] 7.3 实现频率限制
    - 创建 `internal/aggregator/ratelimit.go`：时间窗口内推送次数限制
    - 达到上限后暂存消息，下一窗口按接收顺序批量推送
    - 暂存超过 100 条时合并为摘要推送并清空队列
    - _Requirements: 4.3, 4.5_

  - [x] 7.4 实现聚合器主逻辑
    - 创建 `internal/aggregator/aggregator.go`：组合去重、合并、频率限制
    - 实现 Process 方法：返回 AggregateResult（Pass/Dedupe/Batch/RateLimit）
    - 实现 critical 优先级消息绕过所有聚合规则
    - _Requirements: 4.4_

  - [ ]* 7.5 编写聚合器属性测试
    - **Property 9: 消息去重**
    - **Property 10: 消息批量合并**
    - **Property 11: 频率限制**
    - **Property 12: Critical 消息绕过聚合**
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.4**

- [x] 8. 免打扰调度器
  - [x] 8.1 实现免打扰调度器
    - 创建 `internal/scheduler/dnd.go`：实现 DNDScheduler 结构体
    - 实现 DND 时段编译：解析 HH:MM 格式、处理跨午夜时段、生效日期（everyday/weekday/weekend）
    - 实现 ShouldDefer：判断当前时间是否处于 DND 时段
    - 实现 Enqueue：暂存非 urgent 消息（上限 200 条，超限丢弃最早消息）
    - 实现 FlushPending：DND 结束时按接收时间顺序释放，每条间隔 ≥1 秒
    - 实现重叠 DND 时段合并为连续静默期
    - 允许 urgent 优先级消息直接通过
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_


  - [ ]* 8.2 编写免打扰调度器属性测试
    - **Property 13: 免打扰按优先级分流**
    - **Property 14: 免打扰结束后按时间顺序释放**
    - **Property 15: 重叠 DND 时段合并**
    - **Validates: Requirements 5.2, 5.3, 5.4, 5.5**

- [x] 9. 消息升级器
  - [x] 9.1 实现消息升级器
    - 创建 `internal/escalator/escalator.go`：实现 Escalator 结构体
    - 实现 Track：为需要确认的消息设置升级定时器
    - 实现超时未确认时切换到下一级 Channel 重新推送，标注升级次数和原始发送时间
    - 实现 Acknowledge：收到确认后取消所有待执行升级计划
    - 实现最大升级次数限制：达到上限后执行最终推送并停止
    - 支持固定间隔和递增间隔两种升级策略
    - 实现 Stop：优雅关闭所有定时器
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_

  - [ ]* 9.2 编写消息升级器属性测试
    - **Property 16: 超时未确认触发升级**
    - **Property 17: 升级次数上限**
    - **Property 18: 确认取消升级**
    - **Validates: Requirements 6.1, 6.2, 6.3, 6.4**

- [x] 10. Checkpoint - 确保核心业务逻辑测试通过
  - 确保所有测试通过，如有问题请向用户确认。


- [x] 11. 提醒调度器
  - [x] 11.1 实现提醒调度器
    - 创建 `internal/scheduler/reminder.go`：实现 ReminderScheduler 结构体
    - 实现 Start：从数据库加载所有活跃提醒并注册定时器/cron 任务
    - 实现 Schedule：根据重复类型（once/daily/weekly/cron）设置触发
    - 实现触发时构造内部消息并通过 Router 推送
    - 实现触发后自动计算下一次触发时间（daily +24h、weekly +7d、cron 按表达式）
    - 实现 Cancel：取消指定提醒的定时器
    - 实现推送失败后 1 分钟重试一次，仍失败则标记为触发失败
    - 实现 Stop：优雅关闭
    - _Requirements: 14.2, 14.3, 14.4, 14.9, 14.10_

  - [ ]* 11.2 编写提醒调度器属性测试
    - **Property 24: 提醒调度正确性**
    - **Property 25: 提醒持久化与恢复**
    - **Validates: Requirements 14.2, 14.3, 14.4, 14.9**

- [x] 12. WebSocket Hub
  - [x] 12.1 实现 WebSocket Hub
    - 创建 `internal/ws/hub.go`：实现 Hub 结构体（register/unregister/broadcast channels）
    - 实现 Run：事件循环处理客户端注册/注销/广播
    - 实现 Broadcast：向所有已连接客户端发送事件
    - 实现最大连接数限制（20）
    - 创建 `internal/ws/client.go`：实现客户端连接管理
    - 实现空闲超时检测（60 秒无消息则关闭连接）
    - _Requirements: 16.1, 16.2, 16.3, 16.5, 16.6_

  - [ ]* 12.2 编写 WebSocket 属性测试
    - **Property 27: WebSocket 事件广播**
    - **Validates: Requirements 16.2, 16.3**


- [x] 13. HTTP 服务器与 API 层
  - [x] 13.1 实现 HTTP 服务器与认证中间件
    - 创建 `internal/server/server.go`：Gin 引擎初始化、路由注册、优雅关闭
    - 创建 `internal/server/middleware/auth.go`：API Key 认证中间件
    - 实现 X-API-Key Header 验证，/health 端点豁免
    - 无效/缺失 API Key 返回 HTTP 401 + JSON 错误响应
    - _Requirements: 7.2, 7.3, 11.1, 11.2, 11.5_

  - [ ]* 13.2 编写认证中间件属性测试
    - **Property 7: API Key 认证**
    - **Validates: Requirements 7.2, 7.3, 11.1, 11.2**

  - [x] 13.3 实现 Webhook 处理器
    - 创建 `internal/server/handler/webhook.go`：POST /webhook/{source} 端点
    - 解析 JSON 请求体，验证 title（1-200 字符）、body（1-4000 字符）
    - 构造内部消息（source 来自 URL 路径参数）传递给 Router
    - 成功返回 HTTP 200 + 消息 ID，失败返回 HTTP 400 + 错误详情
    - _Requirements: 7.1, 7.4, 7.5, 7.6, 7.7_

  - [ ]* 13.4 编写 Webhook 属性测试
    - **Property 8: Webhook 请求验证与消息构造**
    - **Validates: Requirements 7.4, 7.5, 7.6, 7.7**

  - [x] 13.5 实现推送 API 处理器
    - 创建 `internal/server/handler/push.go`：POST /api/push 即时推送端点
    - 实现 POST /api/push/test 测试推送端点（不记录历史）
    - 验证请求参数，构造消息传递给 Router
    - _Requirements: 15.1, 15.2, 15.3_


  - [x] 13.6 实现提醒 API 处理器
    - 创建 `internal/server/handler/reminder.go`
    - 实现 POST /api/reminders：创建提醒，验证触发时间不能为过去
    - 实现 GET /api/reminders：查询提醒列表，支持状态筛选
    - 实现 PUT /api/reminders/{id}：修改提醒
    - 实现 DELETE /api/reminders/{id}：取消提醒
    - _Requirements: 14.1, 14.5, 14.6, 14.7, 14.8_

  - [x] 13.7 实现历史查询与统计 API 处理器
    - 创建 `internal/server/handler/history.go`：GET /api/messages 端点
    - 支持时间范围、Channel、状态过滤，分页（默认 20 条，最大 100 条）
    - 验证时间范围不超过 90 天
    - 创建 `internal/server/handler/stats.go`：GET /api/stats 端点
    - 返回各 Channel 推送总数、成功数、失败数、百分比
    - _Requirements: 10.2, 10.3, 10.5, 10.6_

  - [ ]* 13.8 编写统计 API 属性测试
    - **Property 26: 统一 API 响应格式**
    - **Property 28: 统计数据准确性**
    - **Validates: Requirements 10.3, 15.4, 15.5**

  - [x] 13.9 实现健康检查与 WebSocket 端点
    - 创建 `internal/server/handler/health.go`：GET /health 端点，免认证
    - 注册 WebSocket 升级端点 /ws，通过查询参数 token 认证
    - _Requirements: 11.5, 12.6, 16.1, 16.4_

- [x] 14. Checkpoint - 确保 API 层集成测试通过
  - 确保所有测试通过，如有问题请向用户确认。


- [x] 15. 消息处理流水线集成
  - [x] 15.1 串联消息处理流水线
    - 在 server 层将各组件串联：Webhook/Push → Router → Template → Aggregator → Scheduler → Pusher → DB
    - 推送成功后通过 WebSocket Hub 广播事件
    - 推送结果写入数据库（成功/失败/重试次数/失败原因）
    - 实现消息升级器与流水线的集成
    - _Requirements: 10.1, 10.4, 13.5, 16.2_

  - [x] 15.2 实现应用入口与优雅关闭
    - 创建 `cmd/overseer/main.go`：加载配置、初始化各组件、启动服务器
    - 实现 SIGTERM/SIGINT 信号处理：停止接受新请求 → 等待推送完成（30s）→ 关闭 WebSocket → 停止定时器 → 关闭数据库
    - 确保启动时间 < 5 秒
    - _Requirements: 12.6_

- [x] 16. React/Vite 前端与嵌入
  - [x] 16.1 初始化 React/Vite 前端项目
    - 在 `web/` 目录初始化 Vite + React + TypeScript 项目
    - 配置 vite.config.ts：构建输出到 `web/dist`
    - 创建基础页面布局和路由结构
    - _Requirements: 15.1_

  - [x] 16.2 实现前端核心页面
    - 实现推送历史页面：展示消息列表、支持筛选和分页
    - 实现提醒管理页面：创建/修改/取消提醒
    - 实现统计仪表盘：展示各频道推送统计
    - 实现即时推送页面：手动发送推送
    - 实现 WebSocket 实时通知：连接 /ws 端点接收推送事件
    - _Requirements: 15.1, 16.1, 16.2, 16.3_

  - [x] 16.3 实现前端嵌入
    - 创建 `embed.go`：使用 go:embed 嵌入 `web/dist` 静态资源
    - 在 Gin 路由中注册静态文件服务，SPA 路由回退到 index.html
    - _Requirements: 12.1_


- [x] 17. Docker 部署
  - [x] 17.1 编写 Dockerfile 与 docker-compose
    - 创建多阶段构建 Dockerfile：前端构建阶段（Node）→ 后端构建阶段（Go）→ 最终镜像（scratch/distroless）
    - 确保最终镜像 < 50MB
    - 创建 docker-compose.yml：配置 volume 挂载（/data 数据库、/etc/overseer 配置）
    - _Requirements: 12.1, 12.3, 12.4_

- [x] 18. Final Checkpoint - 全量测试与集成验证
  - 确保所有测试通过，如有问题请向用户确认。

## Notes

- 任务标记 `*` 为可选属性测试任务，可跳过以加速 MVP 开发
- 每个任务引用了具体的 Requirements 编号以确保可追溯性
- Checkpoint 任务用于阶段性验证，确保增量开发的正确性
- 属性测试使用 `pgregory.net/rapid` 库，每个属性至少运行 100 次迭代
- 当前阶段仅实现 Bark (iOS) 推送通道，但通过 Pusher 接口预留扩展点
- 前端通过 go:embed 嵌入 Go 二进制，实现单文件部署

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "2.1"] },
    { "id": 2, "tasks": ["2.2", "2.4"] },
    { "id": 3, "tasks": ["2.3", "3.1"] },
    { "id": 4, "tasks": ["3.2", "3.3"] },
    { "id": 5, "tasks": ["3.4", "5.1", "5.4"] },
    { "id": 6, "tasks": ["5.2", "5.3", "5.5", "6.1"] },
    { "id": 7, "tasks": ["6.2", "7.1", "7.2", "7.3"] },
    { "id": 8, "tasks": ["7.4", "7.5"] },
    { "id": 9, "tasks": ["8.1", "9.1"] },
    { "id": 10, "tasks": ["8.2", "9.2", "11.1", "12.1"] },
    { "id": 11, "tasks": ["11.2", "12.2", "13.1"] },
    { "id": 12, "tasks": ["13.2", "13.3", "13.5", "13.6", "13.7", "13.9"] },
    { "id": 13, "tasks": ["13.4", "13.8"] },
    { "id": 14, "tasks": ["15.1", "15.2"] },
    { "id": 15, "tasks": ["16.1"] },
    { "id": 16, "tasks": ["16.2"] },
    { "id": 17, "tasks": ["16.3", "17.1"] }
  ]
}
```
