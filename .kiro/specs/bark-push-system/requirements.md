# Requirements Document

## Introduction

Overseer 是一个基于 Bark 的个人定制推送系统。其核心价值是"听声辨事"——通过自定义声音区分不同类型的通知，让用户无需查看手机即可判断消息的重要性和类别。系统采用 Go 语言开发，SQLite 存储，YAML 配置，Docker 部署，分阶段提供纯 API 和轻量 Web UI。

## Glossary

- **Overseer**: 本推送系统的名称，负责接收、路由、聚合并通过 Bark 推送消息
- **Bark**: iOS 推送服务，支持自定义声音、分组、图标等参数
- **Channel（频道）**: 消息的逻辑分类单元，每个频道绑定一种声音、分组和优先级
- **Router（路由引擎）**: 根据规则将入站消息匹配到对应频道的组件
- **Rule（路由规则）**: 定义消息匹配条件（来源、内容关键词等）与目标频道的映射关系
- **Aggregator（消息聚合器）**: 负责消息去重、批量合并和频率限制的组件
- **Scheduler（免打扰调度器）**: 按时间段控制消息静默与放行的组件
- **Escalator（消息升级器）**: 对未确认的重要消息进行自动升级推送的组件
- **Webhook**: 标准 HTTP 回调入口，接收外部系统（如 GitHub、Grafana）的事件通知
- **Template（消息模板）**: 预定义的消息格式模板，用于将原始数据转换为简洁的推送内容
- **Device_Strategy（多设备策略）**: 控制不同频道推送到不同设备的配置
- **Config_Loader（配置加载器）**: 负责解析和验证 YAML 配置文件的组件
- **DND_Period（免打扰时段）**: 配置中定义的静默时间区间

## Requirements

### Requirement 1: YAML 配置加载与验证

**User Story:** 作为系统管理员，我想通过 YAML 文件配置所有系统参数，以便无需修改代码即可调整系统行为。

#### Acceptance Criteria

1. WHEN Overseer 启动时, THE Config_Loader SHALL 从启动参数或环境变量指定的路径读取 YAML 配置文件并解析为内部配置结构；若未指定路径，则从当前工作目录下的 `config.yaml` 读取
2. IF YAML 配置文件不存在或无读取权限, THEN THE Config_Loader SHALL 返回错误信息，指明文件路径及具体失败原因（不存在或权限不足），并终止启动
3. IF YAML 配置文件包含语法错误, THEN THE Config_Loader SHALL 返回包含行号和错误描述的解析错误信息，并终止启动
4. IF YAML 配置文件缺少必填字段, THEN THE Config_Loader SHALL 返回明确指出缺失字段名称的验证错误信息，并终止启动；必填字段包括：server.port、bark.server_url、bark.device_key
5. THE Config_Loader SHALL 支持解析以下配置段：服务器配置（port 取值范围 1~65535、API key 长度至少 16 个字符）、Bark 配置（server_url、device_key）、频道定义、路由规则、免打扰时间段
6. IF 配置字段值不满足类型或范围约束（如 port 超出 1~65535 范围、server_url 非合法 URL 格式）, THEN THE Config_Loader SHALL 返回错误信息指明字段名称及约束要求，并终止启动
7. THE Config_Loader SHALL 保证 round-trip 一致性：任何通过验证的配置结构经序列化为 YAML 后再次解析，所得配置对象与原对象在所有字段值上逐一相等

### Requirement 2: 频道管理

**User Story:** 作为用户，我想定义多个推送频道并为每个频道绑定独特的声音，以便通过声音区分不同类型的通知。

#### Acceptance Criteria

1. THE Overseer SHALL 支持在 YAML 配置中定义多个 Channel，每个 Channel 包含唯一名称、声音文件名、分组名称、图标 URL 和优先级（active/timeSensitive/passive/critical）属性
2. IF 配置中存在两个或以上同名 Channel, THEN THE Config_Loader SHALL 返回验证错误信息指明重复的 Channel 名称，并终止启动
3. WHEN 消息被路由到某个 Channel 时, THE Overseer SHALL 使用该 Channel 配置的声音参数调用 Bark 推送
4. WHEN 消息被路由到某个 Channel 时, THE Overseer SHALL 使用该 Channel 配置的分组参数对推送进行归类
5. IF Channel 配置中声音字段为空, THEN THE Overseer SHALL 使用 Bark 默认声音进行推送
6. IF Channel 配置中图标字段为空, THEN THE Overseer SHALL 不在推送请求中包含 icon 参数
7. IF 消息指定的 Channel 不存在, THEN THE Overseer SHALL 将消息路由到名为 "default" 的频道
8. IF 名为 "default" 的频道未在配置中定义, THEN THE Overseer SHALL 使用内置默认配置（Bark 默认声音、分组名 "default"、优先级 active）进行推送

### Requirement 3: 路由引擎

**User Story:** 作为用户，我想配置规则让系统自动将消息路由到对应频道，以便无需手动指定每条消息的推送方式。

#### Acceptance Criteria

1. THE Router SHALL 按照配置文件中规则的定义顺序（从上到下）依次匹配入站消息
2. WHEN 入站消息的来源字段与某条 Rule 的 source 条件进行精确字符串匹配成功时, THE Router SHALL 将消息路由到该 Rule 指定的 Channel
3. WHEN 入站消息的 body 字段匹配某条 Rule 的 content 正则表达式条件时, THE Router SHALL 将消息路由到该 Rule 指定的 Channel
4. WHEN 某条 Rule 同时定义了 source 和 content 条件时, THE Router SHALL 仅在两个条件均匹配成功时才将消息路由到该 Rule 指定的 Channel（AND 逻辑）
5. WHEN 入站消息匹配多条 Rule 时, THE Router SHALL 使用第一条匹配的 Rule 进行路由（优先级由配置顺序决定）
6. IF 入站消息未匹配任何 Rule, THEN THE Router SHALL 将消息路由到 "default" 频道
7. IF Rule 的 content 正则表达式语法无效, THEN THE Config_Loader SHALL 在启动时返回验证错误信息指明规则名称和无效的正则表达式，并终止启动

### Requirement 4: 消息聚合与防骚扰

**User Story:** 作为用户，我想让系统自动合并重复消息并限制推送频率，以避免被大量相似通知打扰。

#### Acceptance Criteria

1. WHEN 同一 Channel 在配置的时间窗口（默认 5 分钟）内收到内容完全相同（title 和 body 字段逐字符匹配）的消息时, THE Aggregator SHALL 仅推送第一条消息，并在推送内容中附带当前重复次数
2. WHEN 同一 Channel 在配置的时间窗口（默认 5 分钟）内收到超过配置阈值（默认 10 条）的不同消息时, THE Aggregator SHALL 将这些消息合并为一条摘要推送，摘要内容包含消息总数、时间窗口起止时间和每条消息的 title 列表
3. WHILE 某 Channel 在当前时间窗口（默认 1 小时）内的推送次数达到配置的频率上限（默认 30 次）时, THE Aggregator SHALL 暂存后续消息，并在下一个时间窗口开始时按接收顺序批量推送暂存的消息
4. IF 消息所属 Channel 的优先级为 "critical", THEN THE Aggregator SHALL 跳过所有去重、合并和频率限制规则直接推送该消息
5. IF Aggregator 暂存的消息数量超过 100 条, THEN THE Aggregator SHALL 将暂存消息合并为一条摘要推送并清空暂存队列，摘要内容包含消息总数和各 Channel 的消息计数

### Requirement 5: 免打扰调度

**User Story:** 作为用户，我想设置免打扰时间段，以便在休息时间不被非紧急通知打扰。

#### Acceptance Criteria

1. THE Scheduler SHALL 支持在 YAML 配置中定义最多 10 个 DND_Period，每个时段包含开始时间（HH:MM 24小时制）、结束时间（HH:MM 24小时制）和生效日期（每天/工作日/周末），当开始时间大于结束时间时视为跨午夜时段（如 22:00-07:00 表示当日22:00至次日07:00）
2. WHILE 当前时间处于任一 DND_Period 内时, THE Scheduler SHALL 暂存优先级非 "urgent" 的 Channel 消息，暂存消息上限为 200 条，达到上限后丢弃最早的暂存消息
3. WHILE 当前时间处于 DND_Period 内时, THE Scheduler SHALL 允许优先级为 "urgent" 的 Channel 消息突破静默直接推送
4. WHEN DND_Period 结束时, THE Scheduler SHALL 将暂存期间积累的消息按原始接收时间从早到晚的顺序逐条推送，每条推送间隔不少于 1 秒
5. IF 多个 DND_Period 的生效时间存在重叠, THEN THE Scheduler SHALL 将重叠区间视为连续静默期，在所有重叠时段均结束后再批量推送暂存消息

### Requirement 6: 消息升级

**User Story:** 作为用户，我想让重要消息在未确认时自动升级推送方式，以确保关键信息不被遗漏。

#### Acceptance Criteria

1. WHEN 标记为需要确认的消息在配置的等待时间（1 至 60 分钟）内未收到确认回调时, THE Escalator SHALL 将该消息的推送 Channel 切换到配置中定义的下一级升级 Channel 并重新推送
2. WHEN 消息升级推送时, THE Escalator SHALL 在推送内容中标注当前升级次数（从 1 开始）和原始首次发送时间
3. IF 某消息的升级次数已达到配置的最大升级次数（1 至 10 次）, THEN THE Escalator SHALL 停止该消息的后续升级并执行一次最终推送，内容中标注已达最大升级上限
4. WHEN 收到消息确认回调时, THE Escalator SHALL 取消该消息的所有待执行升级计划
5. WHEN 消息需要再次升级时, THE Escalator SHALL 使用配置的升级间隔时间（固定间隔或递增间隔）等待后再执行下一次升级推送，每次等待时间不超过 120 分钟

### Requirement 7: Webhook 入口

**User Story:** 作为用户，我想通过标准 Webhook URL 接收外部系统的通知，以便集成 GitHub、Grafana 等服务。

#### Acceptance Criteria

1. THE Overseer SHALL 提供 HTTP POST 端点 `/webhook/{source}` 接收外部系统的事件通知，其中 `{source}` 路径参数作为消息的来源标识
2. WHEN 收到 Webhook 请求时, THE Overseer SHALL 验证请求头中的 `X-API-Key` 与配置的密钥一致
3. IF Webhook 请求未携带有效 `X-API-Key`, THEN THE Overseer SHALL 返回 HTTP 401 状态码及包含错误原因的 JSON 响应体并拒绝处理
4. WHEN 收到合法 Webhook 请求时, THE Overseer SHALL 提取请求体内容并构造为内部消息格式传递给 Router
5. THE Overseer SHALL 提供通用 JSON 格式的 Webhook 入口，接受包含 title（必填，1-200 字符）、body（必填，1-4000 字符）、channel（可选）字段的请求体
6. IF Webhook 请求体不是合法 JSON 或缺少必填字段, THEN THE Overseer SHALL 返回 HTTP 400 状态码及包含具体错误原因的 JSON 响应体
7. WHEN Webhook 请求成功处理后, THE Overseer SHALL 返回 HTTP 200 状态码及包含消息 ID 的 JSON 响应体

### Requirement 8: 消息模板

**User Story:** 作为用户，我想定义消息模板来格式化推送内容，以便让不同来源的通知呈现统一简洁的格式。

#### Acceptance Criteria

1. THE Overseer SHALL 支持在 YAML 配置中定义消息模板，每个模板包含唯一名称（1-64 个字母、数字、下划线或连字符字符）和 Go template 格式的内容模板
2. WHEN 路由规则指定了模板名称时, THE Overseer SHALL 使用对应模板渲染消息内容后再推送
3. IF 指定的模板名称不存在, THEN THE Overseer SHALL 使用原始消息内容进行推送并记录警告日志
4. THE Template SHALL 支持访问消息的 source、title、body、timestamp 字段以及 extra 字段（键值对形式，键和值均为字符串）
5. IF 模板渲染过程中发生执行错误（如引用不存在的字段或类型不匹配）, THEN THE Overseer SHALL 回退使用原始消息内容进行推送并记录包含模板名称和错误原因的警告日志
6. IF 模板渲染结果为空字符串, THEN THE Overseer SHALL 使用原始消息内容进行推送并记录警告日志

### Requirement 9: 多设备推送策略

**User Story:** 作为用户，我想让不同频道的消息推送到不同设备，以便在多设备场景下精确控制通知分发。

#### Acceptance Criteria

1. THE Overseer SHALL 支持在 YAML 配置中为每个 Channel 指定 1 至 10 个目标 device_key
2. WHEN Channel 配置了多个 device_key 时, THE Overseer SHALL 向所有配置的设备分别发送推送，并且单个设备推送失败不影响其余设备的推送执行
3. IF Channel 未配置 device_key, THEN THE Overseer SHALL 使用全局默认 device_key 进行推送
4. IF Channel 未配置 device_key 且全局默认 device_key 也未配置, THEN THE Config_Loader SHALL 在启动时返回验证错误信息，指出缺少可用的 device_key 配置
5. IF 向某个 device_key 推送失败, THEN THE Overseer SHALL 对该设备按照 Bark 推送调用的重试策略进行重试，并在所有重试失败后将该设备的推送状态标记为失败并记录到数据库

### Requirement 10: 消息持久化与统计

**User Story:** 作为用户，我想查看推送历史和统计数据，以便了解各频道的使用情况。

#### Acceptance Criteria

1. THE Overseer SHALL 将每条经过 Router 处理的消息持久化存储到 SQLite 数据库，记录字段包含：消息来源（source）、目标频道（channel）、消息标题（title）、消息内容（body）、接收时间戳、推送状态（成功/失败/待推送）
2. THE Overseer SHALL 提供 API 端点查询指定时间范围内的消息推送记录，支持按 Channel 和推送状态筛选，返回结果按时间倒序排列并支持分页（每页默认 20 条，最大 100 条）
3. THE Overseer SHALL 提供 API 端点返回各 Channel 的推送统计数据，包含指定时间范围内每个 Channel 的推送总数、成功数、失败数及其占总推送量的百分比
4. WHEN 消息推送失败时, THE Overseer SHALL 在数据库中记录失败原因描述和已执行的重试次数
5. IF 查询 API 请求的时间范围参数缺失或格式无效, THEN THE Overseer SHALL 返回错误响应，指明无效的参数名称及期望的时间格式
6. IF 查询 API 请求的时间范围超过 90 天, THEN THE Overseer SHALL 返回错误响应，指明允许的最大查询跨度为 90 天

### Requirement 11: API 认证与安全

**User Story:** 作为系统管理员，我想确保 API 接口受到认证保护，以防止未授权访问。

#### Acceptance Criteria

1. THE Overseer SHALL 要求所有 API 请求在 HTTP Header `X-API-Key` 中携带配置的 API key 进行认证
2. IF API 请求未携带 `X-API-Key` Header 或其值与配置的 API key 不一致, THEN THE Overseer SHALL 返回 HTTP 401 状态码及包含错误原因的 JSON 响应体
3. THE Overseer SHALL 支持在 YAML 配置中定义长度不少于 16 个字符的 API key
4. IF YAML 配置中未定义 API key 或 API key 为空字符串, THEN THE Overseer SHALL 在启动时输出错误信息并拒绝启动
5. THE Overseer SHALL 豁免健康检查端点（`/health`）的认证要求，允许未携带 API key 的请求访问该端点

### Requirement 12: Docker 部署

**User Story:** 作为系统管理员，我想通过 Docker 一键部署系统，以便简化运维流程。

#### Acceptance Criteria

1. THE Overseer SHALL 提供多阶段构建 Dockerfile，将 Go 应用编译为单一静态二进制文件，最终镜像基于 scratch 或 distroless 且大小不超过 50MB
2. THE Overseer SHALL 支持通过环境变量 `OVERSEER_CONFIG_PATH` 覆盖默认 YAML 配置文件路径
3. IF 环境变量 `OVERSEER_CONFIG_PATH` 指定的配置文件不存在或不可读, THEN THE Overseer SHALL 在启动时输出包含文件路径的错误信息并以非零退出码终止
4. THE Overseer SHALL 定义容器内默认数据目录 `/data` 用于挂载 SQLite 数据库文件，默认配置目录 `/etc/overseer` 用于挂载配置文件，并支持通过 Docker volume 持久化
5. WHEN Overseer 容器首次启动且 SQLite 数据库文件不存在时, THE Overseer SHALL 自动创建并初始化数据库文件
6. WHEN Overseer 容器启动时, THE Overseer SHALL 在 5 秒内完成初始化，并通过 HTTP 健康检查端点 `/health` 返回成功响应以表明服务就绪

### Requirement 13: Bark 推送调用

**User Story:** 作为用户，我想让系统正确调用 Bark 服务推送消息，以便在 iOS 设备上收到通知。

#### Acceptance Criteria

1. WHEN 消息需要推送时, THE Overseer SHALL 向配置的 Bark server_url 发送 POST 请求，请求体包含 title、body、sound、group、icon 参数
2. IF Bark 服务返回 HTTP 状态码非 200 或响应体中 code 字段值非 200, THEN THE Overseer SHALL 按照指数退避策略重试，初始间隔为 1 秒，每次重试间隔翻倍（1s、2s、4s），最多重试 3 次
3. IF 所有重试均失败, THEN THE Overseer SHALL 将消息标记为推送失败，并将最后一次请求的 HTTP 状态码、响应体和失败时间戳记录到数据库
4. THE Overseer SHALL 在推送请求中设置 10 秒超时时间（包含连接建立和响应读取）
5. WHEN Bark 服务返回 HTTP 200 且响应体中 code 字段值为 200 时, THE Overseer SHALL 将消息标记为推送成功并记录推送时间戳到数据库

### Requirement 14: 提醒与定时任务

**User Story:** 作为用户，我想通过客户端创建定时提醒，以便在指定时间收到推送通知来提醒我做某件事。

#### Acceptance Criteria

1. THE Overseer SHALL 提供 API 端点创建提醒，接受以下参数：标题（title，必填，1-200 字符）、内容（body，可选，0-4000 字符）、触发时间（trigger_at，必填）、频道（channel，可选，默认 "default"）、重复规则（repeat，可选）
2. THE Overseer SHALL 支持以下重复规则类型：一次性（once）、每天（daily）、每周指定星期几（weekly）、自定义 cron 表达式（cron）
3. WHEN 当前时间到达提醒的触发时间时, THE Overseer SHALL 将提醒内容构造为内部消息并通过指定 Channel 推送
4. WHEN 重复提醒触发后, THE Overseer SHALL 根据重复规则自动计算并设置下一次触发时间
5. THE Overseer SHALL 提供 API 端点查询当前用户的所有提醒列表，支持按状态（活跃/已完成/已取消）筛选
6. THE Overseer SHALL 提供 API 端点修改已有提醒的标题、内容、触发时间和重复规则
7. THE Overseer SHALL 提供 API 端点取消指定提醒，取消后该提醒不再触发
8. IF 提醒的触发时间早于当前时间且为一次性提醒, THEN THE Overseer SHALL 返回 HTTP 400 错误，指明触发时间不能为过去时间
9. THE Overseer SHALL 将所有提醒持久化存储到 SQLite 数据库，服务重启后已有提醒不丢失且继续按计划触发
10. WHEN 提醒触发推送失败时, THE Overseer SHALL 在 1 分钟后重试一次，若仍失败则标记为触发失败并记录原因

### Requirement 15: 客户端交互 API

**User Story:** 作为用户，我想通过多种客户端（iOS App、Web UI、Desktop、CLI）与系统交互，以便在任何设备上都能管理我的推送和提醒。

#### Acceptance Criteria

1. THE Overseer SHALL 提供完整的 RESTful API 覆盖以下操作：发送即时推送、创建/修改/取消提醒、查询推送历史、查询统计数据、测试推送
2. THE Overseer SHALL 提供 API 端点 `POST /api/push` 用于手动发送即时推送，接受 title（必填）、body（必填）、channel（可选）、extra（可选，键值对）参数
3. THE Overseer SHALL 提供 API 端点 `POST /api/push/test` 用于发送测试推送到指定设备，不记录到推送历史
4. ALL API 响应 SHALL 使用统一的 JSON 格式：`{"code": 200, "message": "success", "data": {...}}`，错误响应使用对应 HTTP 状态码和错误描述
5. THE Overseer SHALL 在 API 响应中包含分页信息：`{"total": N, "page": P, "page_size": S, "data": [...]}`
6. THE Overseer SHALL 提供 OpenAPI/Swagger 规范文档描述所有 API 端点，便于各客户端开发者参考

### Requirement 16: 多端同步与实时通知

**User Story:** 作为用户，我想在多个客户端之间保持数据同步，并在有新推送时实时收到通知，以便获得一致的使用体验。

#### Acceptance Criteria

1. THE Overseer SHALL 提供 WebSocket 端点 `/ws`，客户端连接后可实时接收新推送事件和提醒状态变更通知
2. WHEN 有新消息推送成功时, THE Overseer SHALL 通过 WebSocket 向所有已连接的客户端广播推送事件（包含消息 ID、标题、频道、时间戳）
3. WHEN 提醒被创建、修改或取消时, THE Overseer SHALL 通过 WebSocket 向所有已连接的客户端广播提醒变更事件
4. THE WebSocket 连接 SHALL 要求在连接建立时通过查询参数 `token` 携带有效的 API key 进行认证
5. IF WebSocket 客户端在 60 秒内未发送任何消息（包括 ping）, THEN THE Overseer SHALL 主动关闭该连接
6. THE Overseer SHALL 支持同时维持最多 20 个 WebSocket 连接
