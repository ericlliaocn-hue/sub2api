# AnyToken 网站 SEO 优化落地方案

适用站点：

- 官网与控制台：`https://anytoken.work`
- 文档站：`https://doc.anytoken.work`
- 模型 API：`https://api.anytoken.work`，仅作为 API 服务域名，不作为内容站提交收录

审计与本地验收日期：2026-08-26

参考范围：当前仓库、当前生产站点、代表性国内外同行公开搜索落地页，以及 `/Users/Zhuanz/lehuo/works/website-templates/seo-guide` 中关于搜索意图、On-Page SEO、canonical、robots、sitemap、内链和搜索平台提交的执行规范。

> P0 抓取基础设施与首批 P1 内容增强已在仓库内落地并完成本地验收，但尚未部署生产。仓库根目录的 `urls.txt` 是经过筛选的 23 条提交候选清单；正式提交前仍应按域名拆分，并再次检查生产 HTTP 状态、canonical 和 `noindex`。

## 1. 目标与原则

### 1.1 业务目标

- 稳定覆盖 `AnyToken`、`AnyToken 官网` 等品牌导航词。
- 以 `AI中转站` 作为首页核心行业词，建立“AnyToken = AI中转站”的品牌与品类关联。
- 用模型广场承接“模型 API 价格、支持模型、Claude/GPT/Grok/Gemini API”等商业调查意图。
- 用公开用量查询页承接“AnyToken API Key 用量、额度、消费查询”等工具意图。
- 用独立文档页承接 API 接入、SDK、CLI 配置、鉴权、流式响应和错误排查等长尾信息词。
- 将自然搜索访问引导到模型广场、注册/登录和文档接入，不让后台页面、支付回调和认证回调参与索引。

### 1.2 工程原则

- 一个搜索意图对应一个规范 URL，避免 `/`、`/home`、`/home-v4`、`/home-classic` 同时竞争首页权重。
- sitemap 只包含规范、可索引、返回 HTTP 200 的 URL。
- 已公开 URL 不随意改名；必须调整时使用 301，并长期保留跳转。
- 每个可索引页面必须有唯一的 `title`、description、H1 和 canonical。
- 结构化数据必须与页面可见内容一致，不生成虚假价格、评分、排行或 FAQ。
- 搜索提交只帮助发现 URL，不保证收录；页面质量、可抓取内容、内链和真实外链仍是核心。

## 2. 基线审计与本地落地状态

### 2.1 架构判断

| 区域 | 实现 | SEO 特征 |
| --- | --- | --- |
| `anytoken.work` | `deploy/nginx/conf.d/anytoken.work.conf` 反向代理到 Go；Vue 3 SPA 由 `backend/internal/web` 嵌入 | 三个公开路由由 Go 输出独立 head、结构化数据与可见 fallback；私有路由 noindex；未知路由真实 404 |
| `doc.anytoken.work` | `docs-site/build.mjs` 从 `pages.json` 生成 20 个独立静态 HTML，`deploy/nginx/conf.d/doc.anytoken.work.conf` 从独立目录直接提供 | 每页有独立 title、description、canonical、Open Graph、可见正文、robots、语义标记和 sitemap |
| `api.anytoken.work` | `deploy/nginx/conf.d/api.anytoken.work.conf` 只代理 API 白名单路由，其他路径返回 JSON 404 | 不应把 API endpoint 当内容页面提交搜索引擎 |

### 2.2 已具备的基础

- 官网首页、模型广场和 API Key 用量查询均有真实公开页面与清晰 H1。
- 首页到模型广场、文档站已有可点击链接。
- 文档站仓库产物已有 20 个独立 URL、完整静态 HTML 和站内导航。
- 文档站已生成并上线：
  - `https://doc.anytoken.work/robots.txt`
  - `https://doc.anytoken.work/sitemap.xml`
  - 每页 canonical、description、Open Graph 和 `index,follow`
- 本轮改动前的生产环境基线已验证：
  - 当时文档站 17 个 sitemap URL 均返回 HTTP 200；新增 3 页仍需部署后复验。
  - 文档站不存在路径返回 HTTP 404。
  - 文档目录无尾斜杠地址会跳转到规范的尾斜杠 URL。
  - HTTP 和 `www.anytoken.work` 会跳转到 HTTPS 裸域。

### 2.3 落地前发现的主要问题

| 优先级 | 问题 | 当前证据 | 影响 |
| --- | --- | --- | --- |
| P0 | 官网没有有效 `robots.txt` | `https://anytoken.work/robots.txt` 返回 `200 text/html` 的 SPA 首页 | 爬虫拿不到明确规则和 sitemap 地址 |
| P0 | 官网没有有效 `sitemap.xml` | `https://anytoken.work/sitemap.xml` 返回 `200 text/html` | 无法稳定提交官网规范 URL |
| P0 | 未知路径返回 200 | 随机不存在路径返回 SPA HTML 和 HTTP 200 | 形成 Soft 404，浪费抓取并污染索引 |
| P0 | 首包 HTML 缺少页面级 SEO 元数据 | `/`、`/model-plaza`、`/key-usage` 首包都只有 `Anytoken - AI API Gateway`，无 description、canonical、OG | 页面主题区分弱，分享摘要和规范化缺失 |
| P0 | 首页存在多个重复入口 | `/` 客户端跳转 `/home`，另有 `/home-v4`、`/home-classic` | 权重和外链信号可能分散 |
| P0 | 非内容路由没有服务端 `noindex` | 登录、注册、回调、支付、后台路由均返回相同 SPA 壳 | 被外链发现后可能进入索引候选 |
| P1 | 官网核心正文依赖 JavaScript 渲染 | Go 只返回通用 `index.html` | 部分爬虫无法稳定理解首页和模型广场内容 |
| P1 | 品牌写法不一致 | 生产设置为 `Anytoken`，文档和页面多处使用 `AnyToken` | 品牌实体与标题一致性不足 |
| P1 | 同 URL 切换中英文 | 当前 i18n 不产生 `/en/` 等独立 URL | 不能正确设置语言级 canonical/hreflang |
| P2 | HTML 内嵌大体积 data URI favicon | 官网首包约 63 KB，文档页约 8 KB，差异主要来自注入的 base64 Logo | 增大所有 SPA 路由首包和爬虫传输成本 |
| P2 | 文档自定义静态 404 未接入 Nginx | `docs-site/build.mjs` 生成 `404.html`，README 要求 `error_page 404 /404.html`，但生产虚拟主机未配置；线上缺失 URL 返回 146 字节的 Nginx 默认 404 | HTTP 状态仍为正确 404，但用户看不到站内返回入口，生成的 noindex 404 未被使用 |
| P2 | 文档静态资源未设置分层缓存，部分 MIME 未纳入 gzip | 文档虚拟主机只有通用 `try_files`；线上 `app.js` 为 `application/javascript` 且未压缩，而 sitemap 能以 gzip 返回 | 不影响收录正确性，但会降低重复访问和文档性能优化空间 |
| P2 | 缺少搜索与转化监控闭环 | 仓库未发现搜索平台验证和自然搜索转化事件 | 无法判断收录、关键词和 CTA 效果 |

### 2.4 本地已落地范围

- 官网 `robots.txt`、三 URL sitemap、23 URL 提交清单由同一份页面清单生成，并支持 `make seo-check` 防漂移。
- `/` 成为唯一首页；`/index.html`、`/home`、`/home-v4`、`/home-classic` 服务端 301 到 `/`。
- `/`、`/model-plaza`、`/key-usage` 首包有独立 title、description、canonical、robots、Open Graph、JSON-LD 和可见 fallback 内容。
- 登录、认证回调、支付、用户后台和管理后台路由服务端输出 `X-Robots-Tag: noindex, follow`；未知路由返回真实 404。
- 文档站新增 OpenAI 兼容迁移、Token 计费、聚合/官方 API 对比 3 页，共 20 个静态页面；补齐 lastmod、上下篇、TechArticle 与 BreadcrumbList microdata。
- 文档 Nginx 接入自定义 noindex 404、短缓存和 gzip MIME，提供可重复的本地 Nginx 验收脚本。
- 尚未完成的工作属于生产部署、搜索平台验证/提交、自然搜索监控、独立 1200×630 分享图、base64 Logo 迁移和供应商主题页，不能在本地冒充完成。

## 3. 可索引页面与 URL 规划

### 3.1 官网规范 URL

| URL | 搜索意图 | 是否进入 sitemap | 备注 |
| --- | --- | --- | --- |
| `https://anytoken.work/` | 品牌、官网、多模型 API 平台 | 是 | 唯一首页 canonical |
| `https://anytoken.work/model-plaza` | 支持模型、模型价格、模型 API 比较 | 是 | 当前生产已公开且无需登录 |
| `https://anytoken.work/key-usage` | API Key 用量、余额、额度查询 | 是 | 工具页，强调 Key 只在浏览器本地处理 |

### 3.2 文档站规范 URL

`docs-site/build.mjs` 当前生成 20 个 URL；保留原有 17 个路径并新增 3 个独立意图页面：

- 文档首页与快速开始：`/`、`/quickstart/`
- 接入指南：`/guides/endpoints/`、`/guides/authentication/`、`/guides/openai-compatible-api/`、`/guides/api-aggregator-vs-direct/`
- API：`/api/responses/`、`/api/chat-completions/`、`/api/models/`、`/api/streaming/`
- SDK：`/sdks/openai/`
- 开发工具：`/tools/codex-cli/`、`/tools/claude-code/`、`/tools/gemini-cli/`、`/tools/opencode/`
- 账户与排错：`/account/billing/`、`/account/token-billing/`、`/troubleshooting/errors/`、`/security/api-keys/`、`/faq/`

### 3.3 明确排除的 URL

以下地址不得进入 sitemap 和批量提交清单，并应输出 `noindex` 或正确状态码：

- 重复首页：`/home`、`/home-v4`、`/home-classic`
- 初始化：`/setup`
- 认证页：`/login`、`/register`、`/email-verify`、`/forgot-password`、`/reset-password`
- OAuth/支付回调：`/auth/**`、`/payment/**`
- 登录后用户页面：`/dashboard`、`/keys`、`/usage`、`/redeem`、`/profile`、`/orders` 等
- 管理页面：`/admin/**`
- 动态私有页：`/custom/:id`
- 当前内容为空且协议功能关闭的 `/legal/terms` 等动态协议页
- API 服务端点：`api.anytoken.work/v1/**`、`/responses`、`/models` 等
- 带跟踪或界面状态参数的 URL，例如 `?utm_*`、`?embedded=1`

其中 `/home*` 建议 301 到 `/`；需要用户操作的页面使用 `noindex,follow`；不存在页面返回真实 404。不要仅依赖 `robots.txt Disallow` 隐藏页面，因为被阻止抓取的页面可能仍以 URL 形式出现在搜索结果中。

## 4. 同行调研与关键词重新设计

### 4.1 调研口径

本轮于 2026-08-26 复核中英文公开搜索结果和代表性同行页面。样本用于分析公开页面的词汇、信息架构和搜索意图，不代表对同行价格、模型数量、稳定性或合规声明的背书。

| 样本 | 公开页面重点 | 可借鉴的信息架构 | AnyToken 不应直接复制的内容 |
| --- | --- | --- | --- |
| [OpenRouter 首页](https://openrouter.ai/) / [模型目录](https://openrouter.ai/models/) / [价格](https://openrouter.ai/pricing) / [Quickstart](https://openrouter.ai/docs/quickstart) | `Unified Interface`、模型目录、价格、单一接入点、快速开始 | 首页承接品类，模型与价格独立，文档解决接入问题 | 模型数量、可用率、路由效果等未经 AnyToken 数据验证的数字 |
| [SiliconFlow 价格中心](https://siliconflow.cn/pricing) | “大模型 API 价格”、输入/输出/缓存价格、厂商和模态筛选 | 将价格比较做成可搜索、可筛选的公开落地页 | “免费”或具体单价不可写死，必须以 AnyToken 实际配置为准 |
| [CometAPI 模型目录](https://www.cometapi.com/models/) / [价格](https://www.cometapi.com/pricing/) | `One API`、模型能力、供应商、价格、统一计费 | 模型目录之外，为价格口径和计费方式提供解释页 | 折扣、官方价差和模型数量不得照搬 |
| [Portkey AI Gateway](https://portkey.ai/docs/product/ai-gateway) / [Model Catalog](https://portkey.ai/docs/product/model-catalog) | AI Gateway、Universal API、模型治理、预算、限流、用量与成本 | 面向工程团队建立网关、模型管理、用量治理的主题集群 | 企业治理能力只有在 AnyToken 对外页面真实可用时才可进入主文案 |
| [TokenRa](https://tokenra.io/zh/) | “大模型 API 聚合平台”“一个 Key”“模型价格对比” | 中文用户能直接理解“一个 Key + 多模型 + 价格” | 折扣、渠道属性和覆盖范围属于动态商业事实，不能静态套用 |
| [RelayAI](https://relayai.com.cn/) | 大模型 API 聚合、OpenAI 兼容、模型价格、企业/合规 | 信任、合规、开票等会影响商业搜索决策 | AnyToken 未公开证明的备案、资质、渠道和 SLA 不得声明 |

同行的共同做法不是反复堆叠模型品牌，而是建立以下路径：

```text
品类首页
  -> 模型/价格目录
  -> 具体协议或模型接入页
  -> 快速开始、计费、安全、错误排查
  -> 注册、创建 Key 或查看模型
```

AnyToken 当前最有条件形成差异的真实内容是：公开模型广场、动态价格/倍率信息、API Key 用量查询、OpenAI/Anthropic/Gemini 等协议文档，以及 Codex CLI、Claude Code、Gemini CLI、OpenCode 的接入文档。关键词策略应围绕这些已存在页面展开，而不是复制同行的“模型最多、价格最低、永久稳定”。

### 4.2 品牌定位与核心词决策

官网统一使用以下搜索定位：

> **AnyToken 是面向开发者的 AI中转站，通过一个 API Key 统一接入当前可用模型，并提供模型价格、用量查询和接入文档。**

核心词选择：

| 词组 | 角色 | 使用位置 | 决策 |
| --- | --- | --- | --- |
| `AnyToken` | 核心品牌词 | 首页 title、首屏、结构化数据、文档站 | **主攻**；统一品牌大小写并与官网、文档、模型广场建立实体关联 |
| `AI中转站` / `AI 中转站` | 核心行业词 | 首页 title、H1、首屏摘要、对比指南 | **主攻**；首页建立品牌与品类关联，对比指南承接解释和决策意图 |
| `多模型 AI API 聚合平台` | 品类解释词 | 首页摘要、正文、对比指南 | **辅助**；用于准确解释统一接入多模型的产品能力 |
| `统一 AI API` / `统一大模型 API` | 解决方案词 | 首页、快速开始、协议页 | **主攻**；对应一个接入点和统一 Key 的开发者需求 |
| `一个 API Key 接入多模型` | 长尾价值词 | 首页首屏、Quickstart、模型广场说明 | **主攻**；自然语言表达，不作为唯一 H1 |
| `AI API 网关` | 工程品类词 | 首页正文、架构/协议文档 | **辅助**；更适合有网关、路由、限流认知的工程用户 |
| `OpenAI 兼容 API` | 协议高意图词 | 独立指南、Quickstart、Chat Completions | **主攻独立页面**；不要只埋在首页 |
| `API 代理` / `转发 API` | 技术或模糊词 | 必要的技术说明 | **不主攻**；容易混入网络代理和安全绕过意图 |

首页采用双核心：品牌词 `AnyToken` + 行业词 `AI中转站`。`多模型 API 聚合平台`、`统一 AI API` 用于解释能力；模型价格、错误码和 CLI 配置继续由各自页面承接，避免首页争夺过多意图。

### 4.3 关键词主题集群

以下是候选词库，不填写未经工具验证的搜索量、KD 或 CPC：

| 集群 | 搜索意图 | 一级关键词 | 二级/长尾词 | 首选承接页 |
| --- | --- | --- | --- | --- |
| 品牌导航 | 找官网、控制台或文档 | AnyToken、AnyToken 官网 | AnyToken API、AnyToken 文档、anytoken.work | `/`；文档首页 |
| 品类商业 | 选择多模型接入平台 | AI中转站 | 多模型 AI API 聚合平台、AI API 聚合平台、统一大模型 API、统一 AI API、AI API 网关、一个 API Key 接入多模型 | `/` |
| 模型与价格 | 比较可用模型和调用成本 | AI 模型 API 价格 | 大模型 API 价格、模型 API 价格对比、Token 价格、输入输出 Token 计费、按量计费、模型列表 | `/model-plaza`；计费说明 |
| 海外模型 | 找具体模型 API | Claude API、GPT API、Gemini API、Grok API | `模型名 + 价格/计费/Base URL/国内接入/OpenAI 兼容/示例` | 未来的供应商主题页；模型广场 |
| 国产模型 | 找具体模型 API | DeepSeek API、Qwen API、Kimi API、GLM API | `模型名 + 价格/模型列表/调用示例/上下文/计费` | 仅在生产实际可用后建设供应商主题页 |
| 协议接入 | 已准备开发，需要可运行代码 | OpenAI 兼容 API | Chat Completions API、Responses API、Anthropic Messages API、Gemini generateContent、流式 API、Function Calling | 对应文档页 |
| 配置参数 | 找到正确地址和鉴权方式 | AI API Base URL | API endpoint、Bearer API Key、Authorization Header、x-api-key、模型列表 API | `/guides/endpoints/`、`/guides/authentication/` |
| 开发工具 | 给现有工具配置模型服务 | Codex CLI API 配置 | Claude Code API 配置、Gemini CLI API 配置、OpenCode API 配置、自定义 Base URL | 4 个现有工具页 |
| 用量工具 | 查询余额和消费 | API Key 用量查询 | API 额度查询、API 消费查询、API Key 余额、请求记录 | `/key-usage` |
| 问题排查 | 请求失败后找原因 | AI API 错误排查 | API 401/403/404/429、model not found、rate limit、流式中断、DNS 错误、Key invalid | `/troubleshooting/errors/`、FAQ |
| 信任决策 | 接入前评估风险 | AI API Key 安全 | 是否存储 Prompt、数据保留、调用日志、Key 泄露、限流、预算、服务状态 | 安全文档；未来的隐私/状态页 |
| 比较教育 | 判断是否适合聚合平台 | AI中转站 | AI中转站是什么、AI 中转站与官方 API 的区别、统一 API 的优缺点、聚合 API 适用场景 | `/guides/api-aggregator-vs-direct/` |

动态模型词的入选门槛：

1. 生产模型广场当前真实可用，而不是代码支持但未配置。
2. 能提供独立的价格口径、协议、模型 ID、能力或调用示例。
3. 页面首包可抓取，有稳定 slug、canonical 和更新时间。
4. 停售后能 301 到供应商主题页或明确返回 410，不留下大量 Soft 404。

不要一次生成数百个只有模型名不同的薄页面。首批建议按供应商主题建设，实际可用且有搜索数据后再细分具体模型。

### 4.4 页面与主关键词映射

| 优先级 | 页面 | 唯一主关键词 | 必须自然出现的相关实体/词 | 不应争夺的词 |
| --- | --- | --- | --- | --- |
| P0 | `https://anytoken.work/` | AnyToken、AI中转站 | 多模型 API 聚合平台、统一 API、API Key、模型价格、用量、开发文档 | 单一模型价格、错误码、CLI 配置 |
| P0 | `https://anytoken.work/model-plaza` | AI 模型 API 价格 | 模型列表、输入/输出 Token、倍率/计费口径、供应商、更新时间 | AnyToken 官网、API 错误排查 |
| P0 | `https://anytoken.work/key-usage` | AnyToken API Key 用量查询 | 额度、消费、请求记录、Key 安全 | 模型价格、API 聚合平台 |
| P0 | `https://doc.anytoken.work/quickstart/` | AnyToken API 快速开始 | 创建 API Key、Base URL、首个请求、模型 ID | 全量协议参考 |
| P0 | `https://doc.anytoken.work/guides/endpoints/` | AnyToken API Base URL | OpenAI/Anthropic/Gemini endpoint、接口地址 | 模型价格 |
| P0 | `https://doc.anytoken.work/guides/authentication/` | API Key 鉴权 | Bearer、Authorization、x-api-key、Key 安全 | 注册优惠 |
| P0 | `https://doc.anytoken.work/api/chat-completions/` | Chat Completions API 示例 | OpenAI 兼容、messages、stream、curl | Responses API 主词 |
| P0 | `https://doc.anytoken.work/api/responses/` | Responses API 示例 | input、output、工具调用、流式响应 | Chat Completions 主词 |
| P0 | 4 个现有 CLI 工具页 | 对应工具名 + API 配置 | Base URL、环境变量、API Key、验证命令 | 其他工具名称 |
| P1 | `https://doc.anytoken.work/guides/openai-compatible-api/` | OpenAI 兼容 API | SDK、Chat Completions、Responses、迁移、差异 | “官方 OpenAI API” |
| P1 | `https://doc.anytoken.work/account/token-billing/` | 大模型 API Token 计费 | 输入、输出、缓存、倍率、账单示例、更新时间 | 固定不变的最低价 |
| P1 | `https://doc.anytoken.work/guides/api-aggregator-vs-direct/` | AI中转站 | AI中转站是什么、与官方 API 的区别、适用场景、成本、迁移、安全 | “最好”“第一名”等自评词 |
| P1 | 首批真实供应商页 | `供应商名 + API` | 当前可用模型、价格、模型 ID、协议、代码、限制 | 当前不可用模型 |

本轮已新增并进入 `urls.txt` 的 URL：

- `https://doc.anytoken.work/guides/openai-compatible-api/`
- `https://doc.anytoken.work/account/token-billing/`
- `https://doc.anytoken.work/guides/api-aggregator-vs-direct/`
- 后续供应商主题页的域名和路径在生产数据快照方案确定后统一；不要先发布客户端空壳 URL。

### 4.5 关键词数据验证与排序

当前未接入百度指数、Google Keyword Planner、Google Search Console、Semrush 或 Ahrefs，因此不虚构搜索量。上线前建立关键词表，至少包含：

| 字段 | 说明 |
| --- | --- |
| `keyword` | 精确关键词，不把多个词写在同一格 |
| `locale/search_engine` | 中文百度/必应/Google 与英文 Google 分开 |
| `intent` | 导航、商业调查、交易、信息、排错、工具 |
| `source` | 搜索联想、相关搜索、Search Console、客服、工单、竞品页面 |
| `target_url` | 唯一承接 URL；为空说明需要新内容 |
| `actual_support` | 生产环境是否真实支持该模型、协议或能力 |
| `volume/KD/CPC` | 来自同一工具、同一地区和同一时间范围的数据 |
| `KDRoi` | `(搜索量 × CPC) / KD`，仅用于同一数据源内相对排序 |
| `business_score` | 是否能引导查看模型、创建 Key、完成调用或充值 |
| `trust_risk` | 是否涉及最低价、稳定性、官方渠道、隐私、合规等证明责任 |
| `status` | 已覆盖、待优化、待建页、观察、放弃 |

排序顺序：生产真实支持 > 搜索意图和页面匹配 > 商业价值 > 可获得难度 > 内容维护成本。即使某模型词热度高，只要当前生产不可用，就不得为了流量建立误导落地页。

### 4.6 文案红线与负面词策略

以下表述只有在有可公开核验的实时数据、合同或合规证据时才能使用：

- `官方直连`、`官方渠道`、`原厂`
- `全网最低价`、`最低至官方 X 折`、`永久免费`
- `100% 稳定`、`永不封号`、`零故障`、`无限量`
- `无需翻墙`、绕过地区限制或账号限制的暗示
- 未公开证明的 SLA、模型数量、用户数量、备案、资质、开票和数据不留存承诺

“ChatGPT API”可以作为用户俗称出现在解释性正文或 FAQ，但正式协议和产品名称使用 `OpenAI API`、具体模型名或 `OpenAI 兼容 API`，避免把 ChatGPT 订阅产品与 API 混为一谈。

执行规则：

- 每页只设一个主要搜索意图；同义词自然覆盖，不机械重复完整关键词。
- title、H1、首段、正文和内链共同表达主题，不添加隐藏关键词或无意义 tag 云。
- 品牌词即使搜索量未知也必须覆盖；品牌统一写作 `AnyToken`。
- 模型、价格和可用性只使用接口真实数据，并展示价格口径与更新时间。
- 新内容优先从 Search Console 查询词、站内搜索、客服问题、错误分类和接入工单提炼。
- 中英文关键词使用独立 URL 后再分别优化；当前同 URL 切换语言时，不做 hreflang 和英文关键词矩阵。

## 5. On-Page SEO 落地

### 5.1 官网标题与摘要

建议将品牌统一为 `AnyToken`，官网三个可索引页使用唯一元数据：

| URL | title | description |
| --- | --- | --- |
| `/` | `AnyToken - AI中转站｜多模型 API 聚合平台` | `AnyToken 是面向开发者的 AI中转站，通过一个 API Key 统一接入当前可用的 Claude、GPT、Grok 等模型，并提供模型价格、用量查询和开发文档。` |
| `/model-plaza` | `AI 模型 API 价格与模型列表｜AnyToken 模型广场` | `查看 AnyToken 当前可用模型及 API 价格，比较输入、输出 Token 计费、倍率、模型能力和可用分组；价格与可用性以页面实时数据为准。` |
| `/key-usage` | `AnyToken API Key 用量查询 - 额度、消费与请求记录` | `在浏览器本地使用 API Key 查询 AnyToken 额度、消费和请求记录；Key 不会被页面存储。` |

要求：

- 服务端首包和客户端路由切换必须输出相同的 title、description 和 canonical。
- 每页保持一个 H1，H2/H3 按真实内容层级组织。
- description 用作摘要，不堆关键词，不使用无法验证的“官方最低价”“永久稳定”等承诺。
- 首页首屏自然出现 `AnyToken`、`AI中转站`、`多模型 API 聚合平台` 和 `统一 API`，模型广场首屏说明数据更新时间和价格口径。

### 5.2 Canonical 规则

```html
<!-- 官网 -->
<link rel="canonical" href="https://anytoken.work/">

<!-- 模型广场 -->
<link rel="canonical" href="https://anytoken.work/model-plaza">

<!-- 文档页示例 -->
<link rel="canonical" href="https://doc.anytoken.work/api/responses/">
```

- `frontend_url` 设置为 `https://anytoken.work`，服务端以该可信配置生成 canonical，不直接信任未经校验的 Host/Forwarded Host。
- `/home`、`/home-v4`、`/home-classic` 301 到 `/`，不依靠 canonical 长期容忍重复页。
- `?embedded=1`、UTM 和其他跟踪参数 canonical 回不带参数的页面。
- 文档页继续保持目录型尾斜杠规范；无尾斜杠使用 301/308 到规范 URL。

### 5.3 Open Graph 与分享卡片

官网三个公开页补齐：

- `og:type`
- `og:site_name`
- `og:title`
- `og:description`
- `og:url`
- `og:image`
- `twitter:card=summary_large_image`

分享图使用固定 HTTPS 文件，例如 `/seo/og-home.png`，建议 1200×630。不要继续把大体积 base64 Logo 注入每个 HTML。

### 5.4 结构化数据

- 首页：`Organization` + `WebSite`；如果页面明确展示 Web 应用能力，可增加 `SoftwareApplication`，`applicationCategory` 使用 `DeveloperApplication`。
- 模型广场：`CollectionPage`；只有服务端首包能输出真实模型列表时才增加 `ItemList`。
- 文档内页：保留 `TechArticle` 语义，并补 `BreadcrumbList` microdata。文档站 CSP 禁止内联脚本，因此不为同一语义额外放宽 CSP 或强行注入 JSON-LD。
- FAQ：仅为页面上真实可见的问答生成 `FAQPage`。
- 不添加虚假 `AggregateRating`、用户数、评价数或价格区间。

### 5.5 内链

- 官网首页正文使用描述性锚文本链接“查看 AI 模型与 API 价格”“阅读 AnyToken API 接入文档”“查询 API Key 用量”。
- 模型广场链接对应的接入文档、鉴权和模型列表 API，不只链接登录页。
- Key 用量查询页链接 API Key 安全文档、计费文档和错误排查。
- 文档站页脚当前已有官网、模型广场和购买入口；建议补“API Key 用量查询”，并将未登录会跳转的购买链接标注为登录后功能。
- 每个文档内页至少有 2-3 个正文相关内链；锚文本说明目的，避免统一使用“点击这里”。

## 6. 技术 SEO 实施任务

### 6.1 P0：先解决抓取与规范化

本节任务已在仓库内完成，本地验收结果见第 9 节；生产部署与站长平台提交仍待执行。

#### A. 生成官网静态 robots 和 sitemap

已落地文件：

- `frontend/public/robots.txt`
- `frontend/public/sitemap.xml`

`robots.txt`：

```txt
User-agent: *
Allow: /

Sitemap: https://anytoken.work/sitemap.xml
```

官网 sitemap 只放三个规范 URL。文档站继续使用自己的 sitemap；不要把两个 Host 的 URL 混入同一个普通 sitemap 文件。

#### B. 统一首页 URL

主要文件：

- `frontend/src/router/index.ts`
- `backend/internal/web/embed_on.go`

实施：

1. 让 `/` 直接渲染正式首页，不再通过客户端 redirect 到 `/home`。
2. `/home`、`/home-v4`、`/home-classic` 在服务端返回 301 到 `/`。
3. 更新站内所有首页链接为 `/`。
4. 保留跳转至少 12 个月，并在搜索平台检查旧 URL 的替换情况。

#### C. 服务端输出路由级 Head

主要文件：

- 新增 `backend/internal/web/seo.go`
- 修改 `backend/internal/web/embed_on.go`
- 修改 `backend/internal/web/html_cache.go`
- 新增 `frontend/src/router/seo.ts` 及测试

实施要点：

- Go 服务端根据已知路由注入 title、description、canonical、robots、Open Graph 和 JSON-LD。
- 原 HTML cache 只有一个全局版本；本轮已改为“品牌化基础 HTML 缓存 + 按请求路径注入 SEO”，并为各路由生成独立 ETag，避免缓存某一路由的 canonical。
- canonical origin 复用已校验的 `frontend_url` 设置；未配置时不从任意 Host 头生成绝对 URL。
- Vue 客户端复用同一份元数据口径，在路由切换后更新 head，避免前后端结果漂移。
- 为三个公开页面和 noindex 路由补单元测试。

#### D. noindex 与真实 404

服务端按路由类别输出：

```http
X-Robots-Tag: noindex, follow
```

适用于认证、用户后台、管理后台、支付、回调和 setup 页面。API JSON 响应可统一 `noindex, nofollow`。

不存在的前端路径仍可返回 Vue 的 404 页面内容，但 HTTP 状态必须为 404；不能继续由 `FrontendServer.Middleware()` 对所有缺失文件一律返回 200。

#### E. 自动生成并校验 URL 清单

将“规范 URL”维护为单一数据源，并由构建脚本生成：

- `frontend/public/sitemap.xml`
- 根目录运营用 `urls.txt`
- 文档站继续由 `docs-site/build.mjs` 生成 `sitemap.xml`

`seo/main-pages.json` 与 `docs-site/pages.json` 是清单数据源；`tools/generate-seo-assets.mjs` 生成官网 robots、sitemap 和根目录 `urls.txt`。`make seo-check` 校验去重、路由、日期与生成产物一致性。

### 6.2 P1：提高首包可理解性

本轮采用 Go 响应期注入的轻量预渲染 fallback，不为控制台引入完整 SSR。三个公开页面首包均包含可见核心说明和内链：

- `/`：输出核心定位、模型广场、文档和用量查询入口；完整交互首页仍由 Vue 接管。
- `/model-plaza`：首包至少输出页面介绍、支持平台说明和更新时间；动态模型列表继续客户端加载，或由服务端使用公开接口安全渲染。
- `/key-usage`：输出工具说明、隐私说明、使用步骤和相关文档内链，不输出任何 Key 或用户数据。

该方案覆盖不执行 JavaScript 的抓取器，同时保持现有 SPA。动态模型列表、Key 查询结果和登录后内容继续客户端/接口加载，不进入 fallback。

### 6.3 P1：文档站增强

主要文件：`docs-site/build.mjs`、`docs-site/index.html`。

静态化决策：继续使用当前构建期静态生成，不为文档站引入完整 SSR 框架。`build.mjs` 已为 20 个规范路由分别输出 `index.html`，同时生成搜索索引、robots、sitemap 和 noindex 的静态 404 页，符合文档 SEO 的主要需求。

| 页面类型 | 渲染策略 | 原因 |
| --- | --- | --- |
| Quickstart、协议、SDK、CLI、鉴权、安全、FAQ、排错 | 构建期静态生成（SSG） | 内容稳定、首包完整、抓取可靠、CDN 缓存简单 |
| 计费原理、倍率说明、聚合/官方 API 对比 | 构建期静态生成（SSG） | 主要是解释性内容，只有正文真实更新时重新构建 |
| 供应商/模型主题页 | 从经过校验的生产数据快照构建静态页 | 保证 title、正文和模型事实一致；展示数据更新时间 |
| 实时模型价格、可用分组、服务状态 | 主站模型广场实时展示；文档静态页只解释口径并链接过去 | 文档域名当前 CSP 为 `connect-src 'none'`，不应为了 SEO 放宽其出网边界 |
| 登录后账户、Key、账单明细、管理后台 | 保持 SPA/接口渲染，并 `noindex` | 私有、个性化内容不应被静态化或收录 |

供应商/模型页构建失败时应中止发布，不能沿用过期或空数据生成可索引页面。价格更新不必每次都重建全部文档；文档链接到主站模型广场读取实时数据，只有标题、支持范围、计费口径或说明正文变化时才更新静态页与 sitemap `lastmod`。

生产 Nginx 配置以 `deploy/nginx/` 为准：

- `doc.anytoken.work.conf` 保留 `try_files $uri $uri/ =404;`，禁止改成 SPA 式 `index.html` 回退。
- 已接入生成的自定义错误页：`error_page 404 /404.html;` 与 internal location 保持最终响应状态为 404。
- `robots.txt`、`sitemap.xml`、HTML 和搜索索引使用短缓存或重新验证；当前 CSS/JS/Logo 文件名未带内容哈希，不得直接设置一年 `immutable`。
- 若后续产物改为内容哈希文件名，再对哈希资源使用一年强缓存；HTML、robots 和 sitemap 仍保持可快速刷新。
- 全局 gzip 已补 `application/javascript application/xml image/svg+xml`，压缩级别由 9 调整为 6；上线前仍需在生产运行 `nginx -t` 并观察 CPU。
- 保持文档站严格 CSP。确有业务需求加载 API 时，应单独评估 CORS、数据暴露与 `connect-src` allowlist，不能为了动态价格直接放宽为 `*`。

- 保持原有 17 个 URL 不变，并新增 3 个独立搜索意图页面。
- 为内页增加 `BreadcrumbList` microdata；继续保持严格 CSP。
- sitemap 可增加真实 `lastmod`；只在正文真实修改时更新，不要每次构建全部刷新。
- 为每页增加清晰的“上一步/下一步”和相关文档内链。
- 新增内容优先级：
  1. OpenAI 兼容 API 迁移指南。
  2. 大模型 API Token 计费、倍率、输入/输出/缓存价格解释。
  3. AI中转站与官方 API 的中立对比，解释行业称呼、适用场景和风险。
  4. Anthropic/Claude Code 与 Gemini CLI 的接入差异和排错。
  5. 429、模型不可用、流式中断和 DNS 的独立问题章节。
  6. 基于生产可用性和真实查询数据建立首批供应商主题页。
- 新页面必须解决不同问题，不生成只有模型名不同的薄内容页。

### 6.4 P2：性能与媒体

- 将后台设置中的 base64 Logo 迁移为同源静态文件或受控 HTTPS URL，避免在每个 SPA HTML 内嵌约 50 KB 以上图片数据。
- 首屏图片提供明确 `width`/`height`；非首屏图片使用 `loading="lazy"`。
- 为大图提供 WebP/AVIF 和合理 `srcset`。
- 为静态哈希资源保留长期缓存，HTML 保持可重新验证；不要缓存动态 canonical 错误版本。
- 上线后以移动端真实数据监控 Core Web Vitals，目标：LCP < 2.5 s、INP < 200 ms、CLS < 0.1（p75）。

### 6.5 P2：多语言

当前中英文共用同一 URL，暂不输出 hreflang。后续确有英文自然搜索需求时：

1. 使用稳定 URL，例如中文默认 `/`、英文 `/en/`，文档对应 `/en/.../`。
2. 每个语言页 canonical 指向自己。
3. 双向输出 `hreflang="zh-CN"`、`hreflang="en"` 和 `x-default`。
4. sitemap 同步记录语言替代页。

不要让所有语言 canonical 都指向中文首页。

## 7. 搜索平台提交

平台功能和配额会变化，以下入口于 2026-08-26 核对可访问；具体配额以登录后的站点后台为准。

| 平台 | 官方入口 | 首次动作 | URL 更新策略 |
| --- | --- | --- | --- |
| Google | `https://search.google.com/search-console` | 建议验证 `anytoken.work` Domain property，提交官网和文档站两个 sitemap | 少量关键页用 URL Inspection；批量使用 sitemap |
| Bing | `https://www.bing.com/webmasters/` | 可从 Search Console 导入，再提交两个 sitemap | 新增/变更内容优先 IndexNow；也支持手动 URL 提交 |
| 百度 | `https://ziyuan.baidu.com/` | 分别验证官网和文档站，使用“普通收录”提交 sitemap/URL | 有真实新增或更新时使用 API/手动提交，按后台配额执行 |
| 搜狗 | `https://zhanzhang.sogou.com/` | 验证站点后检查当前可用提交入口 | 功能资格可能因账号/站点变化，不写死自动推送流程 |
| 360 搜索 | `https://zhanzhang.so.com/` | 验证站点并检查 sitemap/URL 提交能力 | 只提交规范 URL，监控抓取异常 |
| 神马 | `https://zhanzhang.sm.cn/` | 验证移动端站点，核对当前站点提交能力 | 以后台现行能力为准，不假设夸克独立提交接口 |

提交顺序：

1. 先上线 robots、sitemap、canonical、noindex 和 404 修复。
2. 验证两个站点的 sitemap URL 均返回 200 和正确 Content-Type。
3. 在 Google/Bing/百度完成站点验证。
4. 分别提交：
   - `https://anytoken.work/sitemap.xml`
   - `https://doc.anytoken.work/sitemap.xml`
5. 手动优先检查 `/`、`/model-plaza`、文档首页、快速开始、Responses、Chat Completions 和各 CLI 配置页。
6. 不高频重复提交未变化 URL。

`urls.txt` 的定位：

- 是运营和手动批量提交的主清单，一行一个绝对 URL。
- 使用时按 Host 拆分为官网 3 条和文档站 20 条。
- 不能替代两个站点各自的 sitemap。
- Google 与 Bing 支持文本 sitemap，但被托管的文本 sitemap 仍应只包含所属站点的规范 URL。

官方参考：

- Google sitemap：`https://developers.google.com/search/docs/crawling-indexing/sitemaps/build-sitemap`
- Google 请求重新抓取：`https://developers.google.com/search/docs/crawling-indexing/ask-google-to-recrawl`
- Bing sitemap：`https://www.bing.com/webmasters/help/sitemaps-3b5cf6ed`
- Bing URL 提交：`https://www.bing.com/webmasters/help/URL-Submission-62f2860b`
- 百度搜索资源平台使用指南：`https://ziyuan.baidu.com/college/articleinfo/?id=3329`

## 8. 分阶段落地

### PR 1：抓取基础设施（P0，本地已完成）

- 官网 robots、sitemap。
- `/` 唯一首页与旧首页 301。
- 路由级服务端 head。
- noindex 路由分类。
- 真实 404。
- URL/sitemap 自动校验测试。

验收后再向搜索平台提交，不要先提交当前错误的 `/robots.txt` 和 `/sitemap.xml`。

### PR 2：公开页内容与结构化数据（P1，首批本地已完成）

- 首页、模型广场、Key 用量查询预渲染/SSR。
- Organization、WebSite、BreadcrumbList 等真实结构化数据。
- 官网与文档站内链增强。
- 文档 `lastmod` 和相关内容导航。

### PR 3：内容与监控（P1/P2，部分完成）

- 已按同行意图和现有真实能力新增 3 个文档；后续供应商页必须继续用生产数据验证。
- 接入 Search Console、Bing、百度站点监控。
- 记录自然搜索到注册、模型广场、文档 CTA 的匿名聚合转化。
- 优化图片和 Core Web Vitals。

## 9. 验证清单

### 9.1 自动验证

```bash
# 前端
cd frontend
pnpm run lint:check
pnpm run typecheck
pnpm run test:run
pnpm run build

# Go 嵌入前端与 SEO 注入
cd ../backend
go test -tags embed ./internal/web/...

# SEO 生成物与 20 个文档静态页一致性
cd ..
make seo-check

# 文档生产 Nginx 的本地语法与 HTTP 行为
deploy/tests/nginx-doc-seo-test.sh

# 生产 smoke test
curl -sSI https://anytoken.work/robots.txt
curl -sSI https://anytoken.work/sitemap.xml
curl -sSI https://anytoken.work/not-a-real-page
curl -sS https://anytoken.work/model-plaza | rg 'canonical|description|og:title|AI 模型广场'
curl -sS https://doc.anytoken.work/sitemap.xml
curl -sSI https://doc.anytoken.work/not-a-real-page
curl -sS https://doc.anytoken.work/not-a-real-page | rg 'noindex|页面未找到'

# 在生产服务器加载配置前
nginx -t
```

预期：

- `robots.txt` 返回 200 和 `text/plain`。
- `sitemap.xml` 返回 200 和 XML Content-Type。
- 不存在页面返回 404。
- 文档不存在页面返回自定义 `404.html` 的正文、包含 `noindex,follow`，HTTP 状态仍为 404。
- 三个官网公开页首包 title、description、canonical 各不相同。
- 登录、后台、支付和回调页返回 `X-Robots-Tag: noindex`。
- sitemap 中每个 URL 都返回 200，且页面 canonical 等于该 URL。
- `urls.txt` 无重复、无参数、无动态占位符，行尾为 LF。

### 9.2 浏览器验证

- 桌面和窄屏检查首页、模型广场、Key 用量查询。
- 中英文切换后标题和正文一致，不出现错误 canonical/hreflang。
- 复制链接到支持 OG 的调试工具，检查标题、摘要和分享图。
- JavaScript 关闭时，预渲染后的公开页仍有核心介绍和可点击内链。
- Lighthouse 分别检查三个公开页面的 SEO、Accessibility 和 Performance。

### 9.3 搜索平台验收

- Google URL Inspection 显示“用户声明的 canonical”和“Google 选择的 canonical”一致。
- 两个 sitemap 均无解析错误，发现 URL 数符合 3 + 20。
- 百度抓取诊断能读取正文、robots 和 sitemap。
- 每周记录有效索引页、排除原因、品牌词曝光、自然搜索落地页和 CTA 转化。

### 9.4 2026-08-26 本地验收记录

- `go test -tags embed ./internal/web/...`：通过；覆盖公开路由 head/fallback、可信 canonical、旧首页 301、私有路由 noindex、真实 404、robots 与 sitemap 静态响应。
- 前端 `lint:check`、`typecheck`、生产构建：通过；路由 SEO、标题与守卫定向测试 45 项通过。
- 前端全量 Vitest：1673 项通过、5 项失败；失败集中在既有平台配额测试仍假设 5 个平台（当前实现已有 6 个）和跨午夜日期断言，与本轮 SEO 文件无调用关系，未在本任务中扩大范围修复。
- `make seo-check`：通过；官网 3 条、文档 20 条、`urls.txt` 23 条生成物一致。
- `deploy/tests/nginx-doc-seo-test.sh`：通过；覆盖配置语法、文档 200、自定义 404/noindex、canonical、robots Content-Type 与静态资源缓存。
- Playwright/Chromium：通过；首页、模型广场、Key 用量页 metadata 和 JSON-LD 正确，登录页 noindex 且无 canonical；OpenAI 兼容文档有单一 H1、TechArticle 与 BreadcrumbList；官网和文档在 390px 视口均无横向溢出。
- 未验证：生产域名部署后的首包、Nginx 实际证书/模块、搜索平台识别、OG 分享抓取和真实 Core Web Vitals；这些必须在发布后按 9.1-9.3 复验。

## 10. 监控指标

| 周期 | 指标 | 处理阈值 |
| --- | --- | --- |
| 每周 | sitemap 抓取/解析错误 | 任意错误立即处理 |
| 每周 | 404、Soft 404、重复 canonical | 新增异常立即回归路由和跳转 |
| 每周 | 品牌词曝光、点击、CTR、平均排名 | 连续 2-4 周下降时核对标题、索引和站点可用性 |
| 每月 | 自然搜索落地页与注册/登录/文档 CTA | 有曝光无点击先优化摘要；有点击无转化检查意图匹配 |
| 每月 | Core Web Vitals p75 | 任一指标持续不达标进入性能迭代 |
| 每月 | 新增文档的索引与查询词 | 无有效查询且内容重复时合并，不继续扩量 |

## 11. 风险与注意事项

- `Sub2API` 是可自部署开源系统，AnyToken 的域名和品牌 SEO 不应硬编码进通用业务逻辑；生产域名优先从受控设置读取，AnyToken 专属静态资产集中在部署/站点配置层。
- 路由级 HTML 与当前单缓存模型存在冲突，未修改缓存键前不要直接按请求覆盖 canonical。
- sitemap 暴露的是希望公开收录的页面，不得加入后台、动态自定义页、订单、支付结果或任何用户数据 URL。
- 模型价格和可用性会变化，结构化数据和静态文案不得复制过期价格。
- 若未来启用登录协议并填充真实内容，再评估 `/legal/:documentId` 是否进入 sitemap；空文档不能提交。
- 搜索平台验证需要 DNS、站点文件或账号权限，代码侧可完成文件与验证 meta 支持，但最终验证和提交需由持有域名/平台账号的人执行。
