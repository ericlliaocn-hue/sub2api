# AnyToken 微信服务号 AI 内容与自动发布落地方案

适用项目：

- 官网与控制台：`https://anytoken.work`
- 文档站：`https://doc.anytoken.work`
- API：`https://api.anytoken.work`
- 代码仓库：Sub2API / AnyToken 当前仓库

方案日期：2026-08-27

方案状态：待确认后实施

## 1. 决策摘要

AnyToken 公众号采用以下基线方案：

- 账号类型：**企业主体、完成微信认证的服务号**。
- 推荐名称：**AnyToken AI**。
- 核心定位：面向开发者和 AI 用户的模型接入、工具配置、模型选择、成本优化和平台更新服务号。
- 内容频率：每月生产并发布 8 篇文章，每周 2 篇。
- 用户触达：每周组合 1 次主图文/次图文群发，每自然月最多安排 4 次面向用户的群发。
- 自动化边界：AI 自动完成选题、资料收集、事实包、正文、配图、排版、检查、草稿上传和定时发布；群发默认保留人工审批与微信管理员确认。
- 官网导流：新增 `/wx/*` 公开承接页，使用独立营销渠道参数，不复用邀请返利 `aff`。
- 核心指标：公众号来源用户的首次成功 API 调用数，而不是单纯阅读量。

本方案区分两个容易混淆的微信动作：

1. **发布文章**：通过 `/cgi-bin/freepublish/submit` 将草稿发布为公众号文章。
2. **群发消息**：通过 `/cgi-bin/message/mass/sendall` 等接口将内容发送给关注者。

文章发布成功不代表全部关注者都会收到推送。认证服务号的用户每月最多接收 4 条群发消息，因此内容生产频率与群发触达频率必须分别管理。

## 2. 目标与非目标

### 2.1 业务目标

- 建立 `AnyToken` 的官方微信品牌入口。
- 持续产出可搜索、可收藏、能解决真实问题的开发者内容。
- 将公众号访问引导到模型广场、接入文档、注册和首次调用。
- 形成从文章到注册、激活、付费的渠道归因闭环。
- 复用 AnyToken 自身模型和图片能力生成内容，形成真实产品使用案例。
- 降低人工选题、初稿、配图、排版和重复发布操作成本。

### 2.2 工程目标

- 只使用微信官方服务端 API，不使用模拟浏览器登录和点击的 RPA 发布方案。
- 发布、群发、重试和回调处理具备幂等性和多实例安全语义。
- 动态模型、价格、倍率和可用状态来自实时事实快照，不由模型自行编造。
- 内容生成与公众号发布运行在后台 Worker，不阻塞 API 网关请求链路。
- 微信凭据、用户数据和文章素材遵循现有安全、日志脱敏和出网校验约束。
- 提供可审计的选题、来源、审批、发布、群发和失败记录。

### 2.3 非目标

- 不建设通用 CMS 或面向外部客户的公众号 SaaS。
- 不支持批量运营多个无关品牌公众号。
- 不自动转载第三方公众号、新闻媒体或未授权版权内容。
- 不以 AI 自动生成替代事实核验、广告判断和最终运营责任。
- 首期不自动关闭微信 API 群发保护。
- 首期不承诺无人值守群发；无人值守只适用于低风险文章发布。

## 3. 账号与品牌配置

### 3.1 注册要求

- 使用与 AnyToken 业务一致的企业主体注册服务号。
- 完成微信认证后再开始开发接口对接。
- 核对公众号主体、网站备案主体、商标或品牌授权材料的一致性。
- 在公众号后台实际确认草稿、发布、素材、预览、群发和回调权限。
- 设置固定服务器出口 IP 白名单。
- 开启 API 群发保护，首期不为追求无人值守而关闭安全确认。

### 3.2 名称与简介

推荐名称：

> AnyToken AI

名称不可用时按以下顺序选择：

1. `AnyToken开发者`
2. `AnyToken模型服务`
3. `AnyToken AI中转站`
4. `AnyToken模型与工具`

推荐简介：

> AnyToken 官方服务号。一个 API Key 连接 Claude、GPT、Grok 等主流模型，持续分享 API 接入教程、模型评测、开发工具配置、成本优化与平台更新。官网：anytoken.work

审核友好备选简介：

> AnyToken 品牌服务号，分享 AI 模型接入教程、开发工具配置、模型应用、成本优化和平台更新。官网：anytoken.work

品牌 Slogan：

> 一个入口，连接主流 AI 模型。

### 3.3 品牌统一要求

公开品牌统一写作 `AnyToken`：

- 公众号名称、简介和文章作者使用 `AnyToken`。
- 官网 title、页面正文、分享卡片和结构化数据逐步统一为 `AnyToken`。
- `anytoken` 仅用于域名、配置、包名或不区分大小写的技术标识。
- 不在名称和简介中使用 `OpenAI 官方`、`Claude 官方合作` 等无可验证授权的表述。

### 3.4 头像、封面与视觉规范

- 头像沿用 AnyToken 正式 Logo，确保 48px 尺寸仍可识别。
- 封面图建立统一模板，不为每篇文章随机生成完全不同的视觉风格。
- 封面固定包含：栏目标签、短标题、AnyToken 标识。
- 第三方模型品牌以文字为主；使用厂商 Logo 前确认商标使用规范。
- AI 生成图片保留生成来源和必要的显式/隐式标识。
- 封面生成后统一转换为微信接口支持的格式和尺寸，并在上传前压缩。

### 3.5 菜单与关键词回复

建议一级菜单：

| 菜单 | 目标地址 | 作用 |
| --- | --- | --- |
| 模型与价格 | `https://anytoken.work/model-plaza` | 承接模型、价格与分组查询 |
| 开始使用 | `https://anytoken.work/wx/start` | 承接公众号冷流量和注册激活 |
| 接入文档 | `https://doc.anytoken.work` | 承接开发者接入与错误排查 |

建议关键词回复：

| 关键词 | 回复内容 |
| --- | --- |
| 接入 | 快速开始、API Base URL、创建 Key 和首次调用入口 |
| 价格 | 模型广场实时价格入口 |
| Codex | Codex CLI 接入教程 |
| Claude | Claude Code 接入教程 |
| 故障 | 401、403、429、5xx 和模型不可用排查入口 |
| 创作 | 创作中心介绍与登录入口 |
| 交流群 | 当前公开社群联系方式和风险提示 |

## 4. 内容策略

### 4.1 目标受众

- 使用 Codex CLI、Claude Code、OpenCode、OpenAI SDK 等工具的开发者。
- 需要统一管理多个模型和 API Key 的个人或小团队。
- 关注模型价格、Token 成本、缓存和调用稳定性的工程用户。
- 希望通过创作中心生成图片或视频的内容用户。
- 已注册但尚未创建 Key、完成首次调用或充值的用户。

### 4.2 内容栏目与比例

| 栏目 | 比例 | 数据来源 | 主要 CTA |
| --- | ---: | --- | --- |
| 接入实战 | 35% | AnyToken 文档、真实客户端配置、回归验证 | 查看完整文档、完成首次调用 |
| 模型选择 | 25% | 模型广场实时数据、厂商官方文档 | 查看实时模型与价格 |
| 成本与工程实践 | 20% | 用量口径、缓存规则、错误分类、安全实践 | 查看用量、创建 Key |
| AnyToken 更新 | 10% | Git 变更、Release Notes、上线记录 | 体验新功能 |
| 用户案例与 FAQ | 10% | 工单、社群高频问题、匿名化使用场景 | 注册、接入或联系支持 |

### 4.3 发布节奏

每周生产两篇：

- 周二：深度教程、工程实践或模型选择。
- 周四：工具技巧、FAQ、案例或产品更新。

每周群发一次：

- 选择当周价值最高的一篇作为主图文。
- 另一篇作为次图文；实际组合篇数以公众号后台当时允许的规则为准。
- 每自然月最多安排 4 次面向全部用户的群发。
- 紧急故障公告优先通过官网状态、社群和站内公告触达；不要轻易消耗群发额度。

### 4.4 首批 8 周内容日历

| 周次 | 主图文候选 | 次图文候选 | 主要承接页 |
| --- | --- | --- | --- |
| 第 1 周 | 为什么需要一个统一的 AI API 入口？ | 5 分钟接入 AnyToken：从创建 Key 到首次调用 | `/wx/start`、Quickstart |
| 第 2 周 | Codex CLI 接入 AnyToken 完整配置 | Claude Code 接入 AnyToken：Base URL 与模型选择 | Codex/Claude 文档页 |
| 第 3 周 | OpenAI SDK 如何切换到 AnyToken | Chat Completions 和 Responses API 怎么选？ | OpenAI 兼容与协议文档 |
| 第 4 周 | Claude、GPT、Grok：不同任务该选谁？ | 模型价格怎么看：输入、输出、缓存与倍率 | `/model-plaza` |
| 第 5 周 | API 调用遇到 401、429、5xx 怎么排查？ | 一个 API Key 连接多个开发工具 | 错误排查、工具文档 |
| 第 6 周 | 如何控制团队的 API 用量和费用 | 上下文缓存为什么能降低 Token 成本 | 用量与 Token 计费文档 |
| 第 7 周 | API Key 安全清单：这些密钥绝不能放前端 | 创作中心入门：从提示词到图片和视频 | 安全文档、创作中心 |
| 第 8 周 | 如何从模型广场选择适合自己的模型 | AnyToken 月度更新：模型、协议与功能变化 | `/model-plaza`、更新页 |

模型名称只表示内容方向。生成任务必须以发布前的实际模型广场和功能开关为准，不得因为日历中列出某模型就声称其当前可用。

### 4.5 文章统一结构

每篇文章按以下结构生成：

1. 用户问题：说明适用人群和要解决的问题。
2. 先给结果：展示最终效果、验证方式和所需步骤。
3. 实操过程：配置、代码、截图和运行结果。
4. 常见错误：覆盖认证、限流、模型、协议和网络错误。
5. 官网导流：使用与文章意图一致的单一 CTA。
6. 信息声明：更新时间、事实来源、AI 辅助生成和动态信息提示。

文章结尾基线声明：

> 本文由 AnyToken 内容系统辅助生成，经人工审核。模型、价格与可用状态可能发生变化，请以 AnyToken 模型广场实时信息为准。

### 4.6 内容红线

禁止自动生成或发布以下表述：

- `全网最低价`、`永久免费`、`零风险`、`无限量`。
- `100% 稳定`、`永不封号`、`零故障`。
- `官方直连`、`官方渠道`、`原厂`，除非有可公开核验的授权证据。
- 绕过地区限制、账号限制或平台安全策略的引导。
- 当前模型广场、配置或文档中不存在的模型、协议和价格。
- 未经授权的第三方文章大段转载、图片、Logo 或用户案例。
- 明文 API Key、Cookie、OAuth 凭据、用户邮箱、OpenID 或后台敏感截图。

## 5. 自动化总体架构

### 5.1 数据流

```text
内容日历 / 人工选题
          |
          v
可信来源采集器 ----> AnyToken 模型广场 / 文档 / Release Notes
          |           厂商官方文档 / 匿名化高频问题
          v
结构化事实包
          |
          v
AI 大纲、正文、摘要、封面与插图生成
          |
          v
事实校验 / 代码检查 / 去重 / 安全与合规检查
          |
          v
Markdown -> 微信安全 HTML -> 素材上传 -> 微信草稿
          |
          v
管理端预览 -> 人工审批 -> 定时发布
          |
          +----> 文章发布状态查询
          |
          +----> 群发审批 -> 微信管理员确认 -> 群发结果回调
          |
          v
官网访问、注册、首次调用和首单归因
```

### 5.2 部署形态

首期采用现有 Go 服务内的独立后台 Worker：

- HTTP 请求链路只负责管理 API、回调和查询。
- Worker 负责采集、生成、上传、发布、轮询和重试。
- PostgreSQL 是文章、审批、发布和归因状态的事实来源。
- Redis 用于任务唤醒、延迟队列、分布式锁、幂等键和短期 token 缓存。
- 对象存储保存封面、正文图片、生成产物和可审计的来源快照。
- 不使用数据库全表轮询作为主队列；Worker 由 Redis ready/delayed queue 唤醒。

如内容任务负载或安全边界后续明显独立，再拆成单独 Worker 进程；MVP 不提前引入新服务。

### 5.3 模块边界

保持项目既有依赖方向：

```text
handler -> service interface <- repository implementation
```

建议模块：

- `content planning`：内容日历、选题和来源策略。
- `content generation`：事实包、正文、摘要、封面和插图生成。
- `content validation`：事实、代码、链接、重复、敏感信息和合规检查。
- `wechat asset`：正文图片和封面素材上传。
- `wechat draft`：草稿创建、更新、获取和删除。
- `wechat publish`：文章发布和状态查询。
- `wechat mass send`：预览、群发、状态查询和事件回调。
- `marketing attribution`：公众号渠道参数、注册绑定和转化事件。

微信实现位于 repository/adapter 层；service 层不直接依赖微信 HTTP 细节。

## 6. 内容生成设计

### 6.1 可信来源

允许的默认来源：

- AnyToken 模型广场公开 API。
- AnyToken 文档站页面和结构化页面配置。
- AnyToken 当前版本、发布记录和人工维护的 Release Notes。
- OpenAI、Anthropic、Google、xAI 等厂商官方文档。
- 经过匿名化、去除个人信息的工单和社群高频问题。

外部 URL 必须经过协议、域名 allowlist、DNS 解析和私网地址校验，防止采集器成为 SSRF 出口。

默认不允许：

- 任意用户提交 URL 后直接抓取。
- 未注明来源的媒体转载和聚合站数据。
- 社交平台传闻、截图和无法核验的“最新消息”。
- 登录后页面、Cookie 内容和运营人员私人账号数据。

### 6.2 事实包

生成文章前先创建不可变的结构化事实快照：

```json
{
  "topic": "Codex CLI 接入 AnyToken",
  "retrieved_at": "2026-09-01T10:00:00+08:00",
  "site": {
    "base_url": "https://anytoken.work",
    "docs_url": "https://doc.anytoken.work",
    "api_base_url": "https://api.anytoken.work/v1"
  },
  "sources": [
    {
      "url": "https://doc.anytoken.work/tools/codex-cli/",
      "title": "Codex CLI 接入",
      "retrieved_at": "2026-09-01T10:00:00+08:00",
      "content_hash": "sha256:..."
    }
  ],
  "available_models": [],
  "pricing_snapshot": {},
  "supported_protocols": [],
  "product_routes": [],
  "prohibited_claims": []
}
```

规则：

- LLM 只能基于事实包生成动态事实。
- 每条关键事实必须能映射到来源 URL 或内部结构化数据。
- 价格、倍率和可用模型在生成时和发布前各校验一次。
- 来源失效、差异超过阈值或模型状态变化时，文章回退到 `review_required`。
- 事实包保留内容 hash 和抓取时间，便于发布后审计。

### 6.3 模型调用

- 使用 AnyToken 专用运营账号和专用 API Key 调用生产 API，真实验证用户调用链路。
- 专用 Key 设置单独额度、速率、模型白名单和月度预算。
- 文本、代码审查和封面生成使用可配置模型，不在代码中硬编码营销模型名称。
- 生成失败不能自动切换到未批准的高成本模型。
- 不把用户 Prompt、生产密钥或后台私有数据放入内容生成上下文。

### 6.4 图片生成与处理

- 优先复用现有图片异步任务和对象存储能力。
- 封面和插图生成任务与文章 ID 绑定，支持失败重试和人工替换。
- 生成后执行尺寸调整、格式转换、压缩和安全扫描。
- 正文图片必须先通过微信正文图片接口上传，HTML 使用微信返回 URL。
- 封面必须使用微信要求的缩略图素材 `media_id`。
- 原始生成图、处理后图片和微信素材 ID 分开保存。

### 6.5 HTML 渲染

- Markdown 是编辑和审查的源格式。
- 发布前通过固定模板转换为微信兼容 HTML。
- 仅允许明确的 HTML 标签和内联样式白名单。
- 删除 JavaScript、iframe、表单、事件属性和不受信任的外链资源。
- 代码块、表格和长 URL 必须在手机窄屏下可读，不产生横向页面溢出。
- 外链统一通过链接构建器注入渠道参数，不能由 LLM 自由拼接。

## 7. 内容状态机与审批

### 7.1 文章状态

```text
planned
  -> collecting
  -> generating
  -> validating
  -> review_required
  -> approved
  -> scheduled
  -> publishing
  -> published
```

异常与终态：

```text
collecting/generating/validating/publishing -> failed
review_required/approved/scheduled          -> cancelled
published                                  -> archived
```

### 7.2 群发状态

```text
not_planned
  -> planned
  -> awaiting_approval
  -> approved
  -> submitting
  -> awaiting_wechat_confirmation
  -> sending
  -> sent
```

异常终态：

```text
submitting/sending -> failed
awaiting_approval/approved/awaiting_wechat_confirmation -> cancelled/expired
```

文章发布与群发使用独立状态，避免文章已经公开但群发失败时错误回滚文章。

### 7.3 风险等级

| 等级 | 内容 | 自动化策略 |
| --- | --- | --- |
| 低 | 已验证教程修订、月度更新、固定 FAQ | 稳定运行后可自动发布，群发仍审批 |
| 中 | 模型对比、价格分析、产品能力介绍 | 必须人工审批后发布 |
| 高 | 促销、购买引导、合作声明、故障说明、安全事件 | 双人复核，不允许无人值守发布 |

### 7.4 审批要求

- 审批人必须看到渲染后的桌面和手机预览。
- 显示动态事实、来源、数据更新时间和发布前差异。
- 显示文章 CTA、最终 URL、UTM 参数和是否标记广告。
- 高风险文章记录审批人、审批时间和审批意见。
- 已批准文章发生正文、封面、CTA、价格或来源变化后自动撤销批准。

## 8. 微信官方 API 适配

### 8.1 凭据与 token

配置项建议：

```yaml
wechat_content:
  enabled: false
  app_id: ""
  app_secret: ""
  callback_token: ""
  callback_encoding_aes_key: ""
  publish_enabled: false
  mass_send_enabled: false
  source_allowlist: []
  generation_key_id: ""
```

要求：

- `app_secret`、回调 Token、EncodingAESKey 不进入 Git 和普通管理端响应。
- 生产优先通过环境变量或密钥管理注入。
- access token 集中缓存并提前刷新，避免多实例刷新风暴。
- token 刷新使用分布式 singleflight/lock。
- 微信响应与日志经过脱敏，不能输出凭据和完整 OpenID 列表。

### 8.2 接口清单

首期需要接入：

| 能力 | 请求路径 | 用途 |
| --- | --- | --- |
| 稳定凭证 | `/cgi-bin/stable_token` | 获取服务端调用凭证 |
| 正文图片 | `/cgi-bin/media/uploadimg` | 上传文章正文图片并获取微信 URL |
| 封面素材 | `/cgi-bin/material/add_material` | 上传缩略图永久素材 |
| 新建草稿 | `/cgi-bin/draft/add` | 创建微信草稿 |
| 更新草稿 | `/cgi-bin/draft/update` | 审批修改后同步草稿 |
| 获取草稿 | `/cgi-bin/draft/get` | 回读并校验微信草稿 |
| 发布草稿 | `/cgi-bin/freepublish/submit` | 将草稿提交发布 |
| 查询发布状态 | `/cgi-bin/freepublish/get` | 确认最终发布结果 |
| 预览消息 | `/cgi-bin/message/mass/preview` | 向指定运营人员预览 |
| 标签群发 | `/cgi-bin/message/mass/sendall` | 按全体或标签群发 |
| OpenID 群发 | `/cgi-bin/message/mass/send` | 对明确列表群发 |
| 查询群发状态 | `/cgi-bin/message/mass/get` | 主动查询群发结果 |

接口权限以公众号后台和微信当期官方文档为准。任何能力不可用时必须失败关闭，不能自动降级为浏览器模拟操作。

### 8.3 定时发布

定时由 AnyToken Worker 控制：

1. 文章进入 `scheduled` 并保存 `scheduled_publish_at`。
2. Redis delayed queue 在到期后唤醒具体文章 ID。
3. Worker 获取文章级分布式锁。
4. 重新检查审批、事实、微信草稿和功能开关。
5. 调用微信发布接口。
6. 保存 `publish_id` 并轮询最终状态。

不要依赖未在官方接口契约中确认的“定时发布时间”请求参数。

### 8.4 幂等与重试

- 发布幂等键：`wechat_publish:{article_id}:{approved_revision}`。
- 群发 `clientmsgid`：由文章组合、群发范围和计划时间稳定计算，长度符合微信限制。
- 相同审批版本只允许存在一个活动发布任务。
- 网络超时后先查询微信状态，再决定是否重试。
- 429、5xx 和瞬时网络错误使用带抖动的指数退避。
- 参数、权限、原创、广告、安全审核错误不自动无限重试。
- 重试达到上限后进入人工处理并发送告警。

### 8.5 群发保护

微信 API 群发保护可能要求管理员在规定时间内确认。首期策略：

- 系统在计划群发前创建预览并通知审批人。
- 审批通过后调用群发接口。
- 如微信要求管理员确认，管理端显示倒计时和操作说明。
- 超时未确认则记录 `expired`，不自动关闭安全保护或重复提交。
- 文章继续保持已发布状态，运营人员可重新安排下一次群发。

### 8.6 回调安全

- 使用独立公开回调路由，例如 `/api/v1/wechat/content/events`。
- 校验微信签名、时间戳、nonce 和消息加密信息。
- 限制请求体大小并设置读取超时。
- 按 `msg_id/event` 做回调幂等。
- 回调先持久化必要结果，再异步进行统计和告警。
- 未通过签名的请求统一拒绝，不返回内部错误详情。

## 9. 建议数据模型

具体 Ent schema 在实现设计阶段确认。MVP 至少需要以下实体。

### 9.1 `content_articles`

| 字段 | 说明 |
| --- | --- |
| `id` | 内部文章 ID |
| `slug` | 稳定内容标识，不包含 UTM |
| `topic` | 选题 |
| `column` | 内容栏目 |
| `risk_level` | 风险等级 |
| `title` / `digest` | 最终标题和摘要 |
| `content_markdown` | 可编辑源内容 |
| `content_html` | 最终微信 HTML |
| `fact_snapshot` | JSONB 事实快照 |
| `fact_snapshot_hash` | 审批绑定 hash |
| `cover_asset_id` | 内部封面素材 |
| `status` | 文章状态 |
| `revision` | 内容修订号 |
| `approved_revision` | 已批准修订号 |
| `scheduled_publish_at` | 计划发布时间 |
| `published_at` | 实际发布时间 |
| `created_by` / `approved_by` | 创建与审批人 |
| `created_at` / `updated_at` | 审计时间 |

### 9.2 `content_sources`

| 字段 | 说明 |
| --- | --- |
| `article_id` | 所属文章 |
| `url` | 来源 URL |
| `source_type` | AnyToken、厂商官方文档、Release、人工资料 |
| `retrieved_at` | 获取时间 |
| `content_hash` | 来源内容 hash |
| `excerpt` | 支撑关键事实的短摘要，不保存不必要全文 |

### 9.3 `content_assets`

| 字段 | 说明 |
| --- | --- |
| `article_id` | 所属文章 |
| `kind` | cover、inline、chart |
| `object_key` | 内部对象存储 key |
| `mime_type` / `size_bytes` | 文件信息 |
| `ai_generated` | 是否 AI 生成 |
| `wechat_media_id` | 微信素材 ID |
| `wechat_url` | 微信正文图片 URL |
| `metadata` | 标识、尺寸和生成信息 |

### 9.4 `wechat_publications`

| 字段 | 说明 |
| --- | --- |
| `article_id` | 对应文章 |
| `article_revision` | 发布修订号 |
| `draft_media_id` | 微信草稿 ID |
| `publish_id` | 微信发布任务 ID |
| `article_url` | 最终文章 URL |
| `publish_status` | 发布状态 |
| `mass_send_status` | 群发状态 |
| `mass_send_scope` | all、tag、openid list |
| `client_msg_id` | 群发幂等 ID |
| `wechat_msg_id` | 微信群发消息 ID |
| `attempt_count` | 重试次数 |
| `last_error_code` / `last_error_message` | 脱敏错误 |
| `submitted_at` / `completed_at` | 任务时间 |

### 9.5 `content_approvals`

保存文章修订号、风险等级、审批动作、审批人、意见和时间。高风险文章至少两条独立批准记录。

### 9.6 `marketing_attributions`

| 字段 | 说明 |
| --- | --- |
| `anonymous_id` | 首次访问匿名 ID |
| `user_id` | 注册后绑定用户，可空 |
| `source` | `wechat` |
| `medium` | `service_account` |
| `campaign_id` | 活动 ID |
| `article_id` | 文章 ID |
| `content_slot` | menu、main_article、sub_article、footer |
| `first_touched_at` / `last_touched_at` | 首次/末次触点 |
| `registered_at` | 注册时间 |
| `activated_at` | 首次成功调用时间 |
| `first_paid_at` | 首单时间 |

`aff` 是邀请返利代码，具有佣金和财务语义，不能写入或替代 `campaign_id`。

## 10. 管理 API 与前端

### 10.1 管理 API 草案

所有管理接口要求管理员权限；批准、发布、群发和凭据相关操作按风险接入 step-up 认证和审计日志。

```text
GET    /api/v1/admin/wechat-content/articles
POST   /api/v1/admin/wechat-content/articles
GET    /api/v1/admin/wechat-content/articles/:id
PATCH  /api/v1/admin/wechat-content/articles/:id
POST   /api/v1/admin/wechat-content/articles/:id/generate
POST   /api/v1/admin/wechat-content/articles/:id/validate
POST   /api/v1/admin/wechat-content/articles/:id/approve
POST   /api/v1/admin/wechat-content/articles/:id/reject
POST   /api/v1/admin/wechat-content/articles/:id/preview
POST   /api/v1/admin/wechat-content/articles/:id/schedule
POST   /api/v1/admin/wechat-content/articles/:id/publish
POST   /api/v1/admin/wechat-content/articles/:id/mass-send
GET    /api/v1/admin/wechat-content/publications
GET    /api/v1/admin/wechat-content/metrics
```

不提供前端直接调用微信 API 的能力；AppSecret 和 access token 永远留在服务端。

### 10.2 管理页面

建议新增“内容运营”管理模块：

- 内容日历：按周查看选题、状态、计划发布和群发额度。
- 文章编辑：Markdown 编辑、来源、动态事实和 CTA。
- 双端预览：公众号宽度预览和窄屏预览。
- 校验结果：事实差异、代码检查、链接、安全、广告和 AI 标识。
- 审批中心：批准、驳回、意见、修订差异。
- 发布中心：草稿、预览、定时、发布和状态。
- 群发中心：当月额度、受众范围、管理员确认和结果。
- 数据看板：文章访问、注册、激活和首单转化。

页面必须覆盖 loading、empty、error、disabled、任务进行中和移动端窄屏状态；用户可见文案同时维护中英文 locale。

## 11. 官网承接与渠道归因

### 11.1 当前基线

- `/model-plaza` 是公开页面，适合模型和价格内容直接导流。
- `/creation` 和 `/purchase` 需要登录，不适合作为公众号冷流量的第一落点。
- 当前没有公众号专属公开落地页。
- 当前已存在 `aff` 邀请返利代码的 30 天保存逻辑，但没有等价的公众号 UTM 注册归因闭环。

### 11.2 `/wx/*` 路由

建议新增：

```text
/wx/start
/wx/codex-cli
/wx/claude-code
/wx/model-guide
/wx/cost-guide
/wx/changelog
```

路由职责：

- 展示与文章一致的简短价值说明和下一步动作。
- 未登录用户先浏览有效内容，不立刻跳到登录页。
- CTA 再进入注册、模型广场、文档或登录后功能。
- 保留 `source`、`campaign_id`、`article_id` 和 `content_slot`。
- 注册完成后绑定匿名触点和用户 ID。

SEO 处理：

- 有独立、长期内容价值的页面可以索引，并设置唯一 title、description、H1 和 canonical。
- 仅作为重复活动承接的页面使用 `noindex,follow`，canonical 指向对应的长期内容页。
- 带 `utm_*` 的 URL 不进入 sitemap；canonical 去除跟踪参数。
- 不与现有文档站的接入教程制造重复内容竞争。

### 11.3 链接格式

```text
https://anytoken.work/wx/codex-cli
  ?utm_source=wechat
  &utm_medium=service_account
  &utm_campaign=202609_codex_cli
  &utm_content=main_article
```

内部标准化字段：

| UTM | 内部字段 |
| --- | --- |
| `utm_source=wechat` | `source=wechat` |
| `utm_medium=service_account` | `medium=service_account` |
| `utm_campaign` | `campaign_id` |
| `utm_content` | `content_slot` |

所有文章链接由服务端链接构建器产生。LLM 只选择已批准的目标和 CTA 类型，不能手工生成任意 URL。

### 11.4 转化漏斗

```text
wechat_article_published
  -> landing_view
  -> primary_cta_click
  -> register_start
  -> register_success
  -> api_key_created
  -> first_api_call_success
  -> first_paid_order
```

北极星指标：

> 公众号来源、在归因窗口内完成首次成功 API 调用的独立用户数。

辅助指标：

- 文章发布成功率和群发成功率。
- 阅读原文点击率。
- 落地页到注册转化率。
- 注册到创建 Key 转化率。
- 注册到首次成功调用的中位时间。
- 激活到首单转化率。
- 不同栏目和 CTA 的转化差异。
- AI 自动化节省的人工时间、退回率和事实差错率。

前 4 周只建立基线，不使用来源不明的行业平均数作为硬目标；第 5 周起根据自身数据设置改进目标。

## 12. 安全、合规与内容标识

### 12.1 AI 生成内容标识

按《人工智能生成合成内容标识办法》和相关强制性国家标准执行：

- AI 辅助生成文章在文末做显式声明。
- AI 生成图片在适当位置添加可感知标识，并保留文件元数据标识。
- 发布时使用微信提供的 AI 内容声明或标识功能。
- 不删除上游生成服务已添加的标识和元数据。
- 保存生成模型、任务 ID、时间和人工审核记录，避免保存不必要的 Prompt 敏感信息。

### 12.2 广告识别

- 直接推广 AnyToken 并附购买方式的知识、体验或测评内容，进入广告识别检查。
- 需要标记时，在文章显著位置标注“广告”，不能只在文末小字声明。
- 文章前端表述与下一级落地页、价格和购买条件保持一致。
- 促销金额、有效期、适用用户和退款规则必须来自结构化活动配置，不由 LLM生成。

### 12.3 原创与版权

- `send_ignore_reprint` 默认使用失败关闭策略，不因命中原创库而静默继续群发。
- 引用第三方资料只保留必要短摘要并标注来源。
- 不复制第三方公众号排版、封面和大段正文。
- 用户案例必须取得授权并匿名化，不得让 AI 生成虚构客户证言。

### 12.4 安全审查

自动检查：

- API Key、Authorization、Cookie、密码、私钥和 OAuth 凭据。
- 用户邮箱、手机号、OpenID、订单号和后台账号信息。
- 内网地址、管理端 URL、对象存储签名 URL。
- 截图中的密钥、余额、账号和个人信息。
- HTML 中的脚本、事件属性、iframe、外部追踪像素和危险协议。

### 12.5 权限与审计

- 内容查看、编辑、审批、发布和群发使用不同权限点。
- 发布和群发操作进入现有审计日志。
- 高风险文章和群发要求 step-up 认证。
- 凭据修改、回调配置和关闭群发保护属于高风险配置操作。
- 审计记录不保存密钥正文和完整用户受众列表。

## 13. 可观测性与告警

### 13.1 指标

- 任务队列长度、延迟和最老任务时间。
- 采集、生成、校验、上传、发布、群发各阶段成功率和耗时。
- token 获取/刷新成功率和微信 API 错误码分布。
- 发布状态轮询次数、超时和终态分布。
- 群发提交、管理员确认、发送和回调结果。
- 事实差异拦截、敏感信息拦截和广告检查数量。
- 每篇文章的生成 Token、图片成本和总内容成本。

### 13.2 日志

结构化日志允许：

- `article_id`
- `publication_id`
- 脱敏后的微信错误码
- 任务状态、阶段和耗时
- 重试次数

禁止记录：

- AppSecret、access token、回调密钥。
- 完整 OpenID 和用户列表。
- 完整文章正文和事实包中的敏感内容。
- 生成专用 API Key。

### 13.3 告警

- 计划发布时间后仍未发布。
- 发布状态长时间未进入终态。
- 群发等待管理员确认即将超时。
- 微信凭据或接口权限失效。
- 动态事实在发布前发生明显变化。
- 同一文章出现疑似重复发布或群发。
- 当月内容生成预算达到 70%、90%、100%。

## 14. 实施阶段与验收

### 阶段 0：账号与权限确认

交付：

- 企业认证服务号。
- 名称、简介、头像和菜单草案。
- AppID、AppSecret、IP 白名单和回调配置。
- 后台接口权限截图或人工记录。

验收：

- 能获取稳定 access token。
- 能上传一张测试正文图片和封面素材。
- 不向真实关注者群发。

### 阶段 1：官网承接与归因

交付：

- `/wx/start` 和至少两个内容承接页。
- UTM/内部渠道参数保存。
- 注册、创建 Key、首次调用和首单归因字段。
- 基础转化查询或管理看板。

验收：

- 微信参数访问后注册，渠道能绑定到用户。
- 创建 Key、首次成功调用和首单能回溯同一 campaign。
- `aff` 邀请返利逻辑不受影响。
- 带 UTM URL 不进入 sitemap，canonical 正确。
- 浏览器验证桌面/窄屏、深浅色和中英文。

### 阶段 2：内容生成与审核 MVP

交付：

- 内容日历、事实包、文章生成和封面生成。
- Markdown 编辑、双端预览和校验结果。
- 人工批准/驳回和修订绑定。
- 4 篇种子文章。

验收：

- 动态事实全部可以追溯来源。
- 人工修改后可重新校验并生成新修订。
- 价格或模型状态变化会撤销原批准。
- 测试文本中的密钥和危险 HTML 能被拦截。

### 阶段 3：微信草稿、预览与手动发布

交付：

- 素材上传、草稿新增/更新/回读。
- 预览接口。
- 管理员点击发布和状态查询。

验收：

- 微信草稿正文、图片、封面和链接与管理端预览一致。
- 运营人员手机收到预览。
- 测试文章成功发布并获取最终 URL。
- 发布接口超时重试不会生成重复文章。

完成后暂停，请运营人员验证真实微信内阅读、图片、代码块、CTA 和文章链接。

### 阶段 4：定时发布 Worker

交付：

- Redis ready/delayed queue。
- 定时唤醒、分布式锁、幂等和失败重试。
- 发布前事实复核。
- 发布结果告警。

验收：

- 多实例下同一修订只发布一次。
- 重启后已接受任务可恢复。
- 关闭 `publish_enabled` 后不再提交新发布，但历史状态仍可查询。
- 网络超时、微信 5xx、权限错误和内容错误分别按策略处理。

### 阶段 5：群发与回调

交付：

- 月度额度展示。
- 预览、审批、群发提交、管理员确认状态和事件回调。
- `clientmsgid` 幂等。

验收：

- 先仅对测试标签/OpenID 群发。
- 同一 `clientmsgid` 重试不会重复触达。
- 回调签名错误被拒绝。
- 成功、审核失败、管理员拒绝和确认超时均正确落库。
- 全量群发前由用户明确确认真实内容、受众和时间。

### 阶段 6：受控自动发布

开放条件：

- 至少连续 8 周无严重事实或安全事故。
- 发布成功率和状态恢复达到运营要求。
- 低风险模板退回率处于可接受范围。
- 运营确认哪些栏目允许自动发布。

仅低风险文章可以从 `approved` 自动进入定时发布；模型对比、价格、促销、合作、安全和故障内容继续人工审批。群发不随文章自动发布权限一起放开。

## 15. 测试策略

### 15.1 单元测试

- 文章与群发状态机。
- 审批修订绑定和变更后撤销。
- UTM/渠道链接构建。
- 事实包 hash 和发布前差异。
- HTML 白名单与敏感信息检测。
- token 缓存、提前刷新和并发 singleflight。
- 幂等键和 `clientmsgid` 计算。
- 微信错误码分类和重试策略。

### 15.2 集成测试

使用 fake WeChat server 覆盖：

- token 成功、过期、刷新和权限错误。
- 图片和素材上传。
- 草稿新增、更新和回读差异。
- 发布提交成功但最终失败。
- 网络超时后查询已成功，防止重复提交。
- 群发成功、原创失败、涉嫌广告、管理员拒绝和确认超时。
- 回调签名、加解密和重复事件。

PostgreSQL/Redis 集成测试覆盖：

- 多 Worker 竞争同一文章。
- delayed queue 到期与重启恢复。
- 发布成功后的事务和状态一致性。
- 回调与主动轮询同时完成的并发收敛。

### 15.3 前端测试

- 内容列表、编辑、预览、审批、定时和群发交互。
- loading、error、empty、disabled 和终态。
- 风险提示和二次确认。
- 双语文案和移动端窄屏。
- 链接和渠道参数展示。

### 15.4 真实环境验收

真实微信环境不可由单元测试替代，必须验证：

1. 手机微信内的字体、代码块、图片和深色模式表现。
2. 阅读原文和菜单能打开官网 HTTPS 页面。
3. 微信内浏览器注册、登录、OAuth 和支付链路。
4. 测试标签/OpenID 群发与回调。
5. 管理员确认流程和超时。
6. 正式群发前的受众、时间和文章组合。

## 16. 失败处理与回滚

### 16.1 功能开关

- `wechat_content.enabled`：总开关。
- `wechat_content.publish_enabled`：禁止新的发布提交。
- `wechat_content.mass_send_enabled`：禁止新的群发提交。
- 关闭开关不删除文章、草稿、历史任务和审计记录。
- 已提交微信的任务继续查询终态，不能因关闭开关而丢失状态。

### 16.2 故障策略

| 故障 | 行为 |
| --- | --- |
| 事实来源不可用 | 停止生成/发布，进入人工审核 |
| 生成模型不可用 | 有界重试或使用已批准降级模型，不静默扩大成本 |
| 图片生成失败 | 允许人工替换或使用品牌模板，不发布空封面 |
| 微信 token 失败 | 刷新一次后失败关闭并告警 |
| 发布结果未知 | 查询状态，不直接重复提交 |
| 群发确认超时 | 标记 expired，保留已发布文章，不自动重发 |
| 原创/广告审核失败 | 停止自动流程，展示微信原因并人工处理 |
| 归因服务失败 | 不阻断官网访问和注册，异步补偿归因事件 |

### 16.3 内容纠错

- 已发布文章发现事实错误时立即停止后续群发。
- 根据微信能力修改、删除或发布更正说明；删除属于不可逆操作，必须人工确认。
- 记录错误原因、影响文章、修复动作和规则更新。
- 同类错误加入自动校验回归用例。

## 17. 上线检查清单

### 账号

- [ ] 企业主体服务号完成微信认证。
- [ ] 名称、简介、头像和品牌大小写确认。
- [ ] AppID、AppSecret、IP 白名单和回调配置完成。
- [ ] 草稿、发布、素材、预览、群发权限逐项确认。
- [ ] API 群发保护策略确认。

### 内容

- [ ] 8 周内容日历确认。
- [ ] 4 篇种子文章通过人工审核。
- [ ] 文章模板、封面模板和 CTA 规范确认。
- [ ] AI 内容标识和广告判断流程确认。
- [ ] 第三方商标、图片和引用规则确认。

### 工程

- [ ] `/wx/*` 落地页和归因完成。
- [ ] `aff` 与公众号渠道参数完全隔离。
- [ ] 事实包、生成、校验、审批和修订完成。
- [ ] 微信 token、素材、草稿、预览和发布完成。
- [ ] 幂等、重试、状态查询和告警完成。
- [ ] 测试标签群发和回调完成。
- [ ] 审计日志和 step-up 权限完成。
- [ ] 中英文文案、移动端和微信内浏览器验收完成。

### 正式群发

- [ ] 文章已在真实微信环境预览。
- [ ] 模型、价格和链接在群发前再次确认。
- [ ] AI/广告/来源标识已确认。
- [ ] 群发范围、预计人数和当月额度已确认。
- [ ] `clientmsgid` 和计划时间已确认。
- [ ] 管理员在确认窗口内可操作。

## 18. 主要风险与取舍

| 风险 | 影响 | 缓解措施 |
| --- | --- | --- |
| 微信接口权限或规则变化 | 自动发布/群发中断 | 启动时能力探测、后台显示权限、失败关闭、人工草稿兜底 |
| AI 生成动态事实错误 | 品牌和合规风险 | 事实包、双重实时校验、来源审计、人工审批 |
| 服务号每月群发额度有限 | 内容不能全部主动触达 | 每周主/次图文组合，其他文章通过发布、菜单、社群和 SEO 分发 |
| 管理员确认导致无法完全无人值守 | 定时群发可能超时 | 文章自动发布与群发解耦，群发排班和倒计时告警 |
| 发布接口超时导致重复 | 重复文章或消息 | 稳定幂等键、状态查询优先、`clientmsgid` |
| 官网归因与返利混用 | 财务和数据错误 | `campaign_id` 与 `aff` 独立建模和测试 |
| 微信 HTML 与浏览器渲染差异 | 真实阅读体验下降 | 固定模板、手机预览、真实微信验收 |
| 自动采集成为 SSRF 出口 | 内网和凭据风险 | 域名 allowlist、协议/解析/私网校验、限流和超时 |
| 内容成本失控 | 运营费用异常 | 专用 Key、模型白名单、预算、单篇上限和告警 |

## 19. 完成标准

本方案完成实施必须同时满足：

- 企业认证服务号的官方草稿、发布、预览和群发能力已在真实账号验证。
- 公众号文章可以从选题进入事实包、AI 生成、校验、审批、草稿、预览、定时发布和状态查询完整链路。
- 发布和群发在网络超时、多实例和重复回调下不会重复执行。
- 动态模型、价格和协议可以追溯到生成时和发布前的真实来源。
- 官网 `/wx/*` 可以承接公众号访问，并完成注册、首次调用和首单归因。
- 公众号渠道不影响现有邀请返利和支付语义。
- AI 内容标识、广告判断、原创校验、敏感信息检查和人工审批可审计。
- 自动化测试通过，并完成手机微信内、测试标签群发和管理员确认的真实验收。
- 未经用户明确确认，不进行面向全部真实关注者的首次正式群发。

## 20. 参考资料

- 微信服务号草稿管理：<https://developers.weixin.qq.com/doc/service/guide/product/draft.html>
- 微信服务号发布能力：<https://developers.weixin.qq.com/doc/service/guide/product/publish.html>
- 微信服务号群发消息：<https://developers.weixin.qq.com/doc/service/guide/product/message/Batch_Sends.html>
- 《人工智能生成合成内容标识办法》：<https://www.cac.gov.cn/2025-03/14/c_1743654684782215.htm>
- GB 45438—2025《网络安全技术 人工智能生成合成内容标识方法》：<https://openstd.samr.gov.cn/bzgk/std/newGbInfo?hcno=F32EA2A561F1886CD8D606513512D547&refer=outter>
- 《互联网广告管理办法》：<https://www.samr.gov.cn/zw/zfxxgk/fdzdgknr/fgs/art/2023/art_d93a579afd45413e8576e4623fab348f.html>
- AnyToken 官网：<https://anytoken.work>
- AnyToken 文档站：<https://doc.anytoken.work>

