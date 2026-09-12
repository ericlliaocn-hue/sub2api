export default {
  batchImageGuide: {
    title: '图片批量生成',
    description: '一次提交多条提示词，任务完成后可统一下载图片结果'
  },
  // Home Page
  home: {
    viewOnGithub: '在 GitHub 上查看',
    viewDocs: '查看文档',
    docs: '文档',
    switchToLight: '切换到浅色模式',
    switchToDark: '切换到深色模式',
    dashboard: '控制台',
    login: '登录',
    getStarted: '立即开始',
    goToDashboard: '进入控制台',
    // 新增：面向用户的价值主张
    heroSubtitle: '一个密钥，畅用多个 AI 模型',
    heroDescription: '无需管理多个订阅账号，一站式接入 Claude、GPT、Gemini 等主流 AI 服务',
    tags: {
      subscriptionToApi: '订阅转 API',
      stickySession: '会话保持',
      realtimeBilling: '按量计费'
    },
    // 用户痛点区块
    painPoints: {
      title: '你是否也遇到这些问题？',
      items: {
        expensive: {
          title: '订阅费用高',
          desc: '每个 AI 服务都要单独订阅，每月支出越来越多'
        },
        complex: {
          title: '多账号难管理',
          desc: '不同平台的账号、密钥分散各处，管理起来很麻烦'
        },
        unstable: {
          title: '服务不稳定',
          desc: '单一账号容易触发限制，影响正常使用'
        },
        noControl: {
          title: '用量无法控制',
          desc: '不知道钱花在哪了，也无法限制团队成员的使用'
        }
      }
    },
    // 解决方案区块
    solutions: {
      title: '我们帮你解决',
      subtitle: '简单三步，开始省心使用 AI'
    },
    features: {
      unifiedGateway: '一键接入',
      unifiedGatewayDesc: '获取一个 API 密钥，即可调用所有已接入的 AI 模型，无需分别申请。',
      multiAccount: '稳定可靠',
      multiAccountDesc: '智能调度多个上游账号，自动切换和负载均衡，告别频繁报错。',
      balanceQuota: '用多少付多少',
      balanceQuotaDesc: '按实际使用量计费，支持设置配额上限，团队用量一目了然。'
    },
    // 优势对比
    comparison: {
      title: '为什么选择我们？',
      headers: {
        feature: '对比项',
        official: '官方订阅',
        us: '本平台'
      },
      items: {
        pricing: {
          feature: '付费方式',
          official: '固定月费，用不完也付',
          us: '按量付费，用多少付多少'
        },
        models: {
          feature: '模型选择',
          official: '单一服务商',
          us: '多模型随意切换'
        },
        management: {
          feature: '账号管理',
          official: '每个服务单独管理',
          us: '统一密钥，一站管理'
        },
        stability: {
          feature: '服务稳定性',
          official: '单账号易触发限制',
          us: '多账号池，自动切换'
        },
        control: {
          feature: '用量控制',
          official: '无法限制',
          us: '可设配额、查明细'
        }
      }
    },
    providers: {
      title: '已支持的 AI 模型',
      description: '一个 API，多种选择',
      supported: '已支持',
      soon: '即将推出',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: '更多'
    },
    // CTA 区块
    cta: {
      title: '准备好开始了吗？',
      description: '注册即可获得免费试用额度，体验一站式 AI 服务',
      button: '免费注册'
    },
    footer: {
      allRightsReserved: '保留所有权利。'
    },
    v2: {
      eyebrow: 'AI 服务中转站 · 一站式入口',
      title: '把想用的 AI，',
      accent: '放在一个入口里',
      description: '模型选择、订阅使用、图片视频创作与用量管理，都从这里开始。按需选择，清楚消费，随时切换你的 AI 工作方式。',
      primaryAction: '浏览模型与价格',
      secondaryAction: '进入创作中心',
      nav: {
        models: '模型广场',
        creation: '创作中心',
        pricing: '订阅 / 充值'
      },
      trust: {
        models: '主流模型可选',
        billing: '价格与用量清楚',
        creation: '图片视频都能做'
      },
      workspace: {
        label: 'personal workspace',
        ready: 'ready to use',
        greeting: '你的 AI 工作台',
        title: '今天想做点什么？',
        models: '模型选择',
        creation: '创作入口',
        usage: '用量可查',
        featured: '常用模型',
        available: '可直接使用'
      },
      flow: {
        choose: '选模型',
        use: '开始使用',
        track: '看用量'
      },
      float: {
        fastTitle: '快速开始',
        fastDesc: '选好就能用',
        clearTitle: '账单清楚',
        clearDesc: '余额与用量可查'
      },
      features: {
        label: '你的使用方式',
        title: '不管你是调用模型，还是直接创作',
        description: '把复杂的模型、渠道和计费交给平台，你只需要选择适合自己的使用方式。',
        modelsTitle: '先逛模型，再决定怎么用',
        modelsDesc: '在模型广场查看可用模型、分组和实际价格，找到适合你的选择。',
        creationTitle: '打开就能创作',
        creationDesc: '进入创作中心，选择图片或视频模型，用自己的素材和想法直接产出。',
        usageTitle: '每一笔用量都看得见',
        usageDesc: '余额、订阅、API Key 和调用明细集中管理，使用多少心里有数。'
      },
      modules: {
        label: '平台能力',
        title: '从第一次尝试，到长期使用',
        modelsTitle: '模型广场',
        modelsDesc: '浏览模型、价格与可用分组',
        subscriptionTitle: '订阅与充值',
        subscriptionDesc: '按自己的节奏购买服务与额度',
        creationTitle: '图片 / 视频创作',
        creationDesc: '把提示词和素材变成作品',
        keysTitle: 'API Key 与用量',
        keysDesc: '给工具和应用接入统一的调用入口'
      },
      cta: {
        label: '现在开始',
        title: '先看看有哪些模型适合你。',
        description: '不用先理解复杂配置，打开模型广场，从价格和能力开始选择。',
        action: '进入模型广场'
      },
      footer: '让 AI 使用更简单'
    },
    v3: {
      nav: {
        models: '模型广场',
        creation: '创作中心',
        pricing: '定价'
      },
      hero: {
        kicker: 'AI 中转站 · 一个入口，很多可能',
        line1: '不再为 AI',
        line2: '东奔西跑。',
        line3: '从选择开始。',
        description: '把模型、创作和消费放进同一个轻盈的工作流。先找到适合你的能力，再用最舒服的方式把想法做出来。',
        primary: '逛逛模型广场',
        secondary: '去创作中心',
        proofModels: '多模型可选',
        proofPricing: '价格透明',
        proofCreation: '直接产出'
      },
      map: {
        caption: 'your AI route map',
        models: '模型广场',
        modelsHint: '比较能力与价格',
        creation: '创作中心',
        creationHint: '图片与视频',
        pricing: '定价与充值',
        pricingHint: '按需选择额度',
        apiHint: '给工具一个入口',
        status: 'all systems ready'
      },
      rail: { label: '可选择的能力' },
      choices: {
        label: '从这里开始',
        title: '你的 AI 工作方式，不必只有一种。',
        description: '今天想试一个新模型，还是把一个灵感变成作品？把入口交给你，把复杂度留给平台。',
        modelsMeta: 'EXPLORE / COMPARE',
        modelsTitle: '先选模型',
        modelsDesc: '看能力、分组和实际价格，找到真正适合你的那一个。',
        creationMeta: 'MAKE / CREATE',
        creationTitle: '直接创作',
        creationDesc: '从提示词和素材出发，快速生成图片或视频。',
        pricingMeta: 'PLAN / RECHARGE',
        pricingTitle: '按自己的节奏使用',
        pricingDesc: '订阅、充值、余额和用量都清楚摆在眼前。'
      },
      promise: {
        label: '我们把复杂藏起来',
        title: '你只需要做选择。',
        description: '模型怎么接入、渠道怎么切换、每笔用量怎么算，由平台处理；你只关注正在做的事。',
        item1Title: '一个入口，随时切换',
        item1Desc: '不同模型和能力集中在模型广场，不用来回切换多个平台。',
        item2Title: '看得懂的价格',
        item2Desc: '从订阅到按量使用，余额、额度和消费记录都保持清晰。',
        item3Title: '从灵感到成品',
        item3Desc: '创作中心直接连接图片与视频工作流，让想法更快落地。'
      },
      cta: {
        label: '现在就开始',
        title: '先找到下一个值得使用的 AI。',
        description: '打开模型广场，从能力和价格开始选择；喜欢哪一种，再把它变成你的日常工作流。',
        action: '进入模型广场'
      },
      footer: '模型、创作与用量，一个更舒服的入口'
    },
    v4: {
      nav: {
        models: '模型广场',
        creation: '创作中心',
        pricing: '定价'
      },
      hero: {
        meta: 'OpenAI · Anthropic · Gemini 协议',
        title: '一个地址，\n所有模型。',
        accent: '换个 base_url 的事。',
        description: '把请求发到这里，Claude、GPT、Gemini、Grok 原样可用。一把 Key，一份余额，每一次调用都有账可查。',
        primary: '看模型与价格',
        secondary: '拿一把 Key',
        ioIn: '进',
        ioOut: '出'
      },
      three: {
        label: '三件事',
        n1Title: '一个地址',
        n1Desc: '把 base_url 指向这里。OpenAI、Anthropic、Gemini 三种协议原样可用，SDK 和 CLI 都不用换。',
        n2Title: '一把 Key',
        n2Desc: '所有模型共用一把 Key、一份余额，不用在几家之间来回充值和切换。',
        n3Title: '一份账单',
        n3Desc: '每次调用的模型、Token 与费用都记着。凭 Key 就能查，不登录也行。'
      },
      models: {
        label: '模型',
        title: '今天能调的模型',
        note: '具体型号、分组与实时价格，以模型广场为准。',
        more: '全部模型与价格'
      },
      code: {
        label: '接入',
        title: '把 base_url 换过来，',
        accent: '剩下的不用改。',
        description: '官方 SDK、Codex CLI、Claude Code 和常见客户端都支持自定义地址。',
        tabCurl: 'curl',
        tabPython: 'Python',
        tabNode: 'Node.js',
        baseLabel: '接入地址',
        copy: '复制',
        copied: '已复制',
        prompt: '你好'
      },
      faq: {
        label: '常见问题',
        q1: '支持哪些协议和客户端？',
        a1: 'OpenAI Chat Completions、Responses API、Anthropic Messages 和 Gemini generateContent。凡是允许自定义 base_url 的 SDK、CLI 和客户端都能直接用。',
        q2: '怎么计费？',
        a2: '按实际用量计费，价格按模型区分，以模型广场为准。充值后余额通用于所有模型。',
        q3: '怎么查用量？',
        a3: '登录后在用量页看明细；也可以在「API Key 用量查询」页直接输入 Key，不需要登录。'
      },
      cta: {
        title: '现在就换。',
        description: '先看模型与价格，再拿一把 Key，两分钟的事。',
        action: '看模型与价格'
      },
      footerUsage: 'API Key 用量查询',
      footer: '一个地址，所有模型'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key 用量查询',
    subtitle: '输入您的 API Key 以查看实时消费金额与使用状态',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: '查询',
    querying: '查询中...',
    privacyNote: '您的 Key 仅在浏览器本地处理，不会被存储',
    relatedPages: '相关页面',
    securityGuide: 'API Key 安全',
    billingGuide: '用量与计费',
    errorGuide: '错误排查',
    dateRange: '统计范围:',
    dateRangeToday: '今日',
    dateRange7d: '7 天',
    dateRange30d: '30 天',
    dateRange90d: '90 天',
    dateRangeCustom: '自定义',
    apply: '应用',
    used: '已使用',
    detailInfo: '详细信息',
    tokenStats: 'Token 统计',
    dailyDetail: '按日明细',
    modelStats: '模型用量统计',
    // Table headers
    date: '日期',
    model: '模型',
    requests: '请求数',
    inputTokens: '输入 Tokens',
    outputTokens: '输出 Tokens',
    cacheCreationTokens: '缓存创建',
    cacheReadTokens: '缓存读取',
    cacheWriteTokens: '缓存写入',
    totalTokens: '总 Tokens',
    cost: '费用',
    // Status
    quotaMode: 'Key 限额模式',
    walletBalance: '钱包余额',
    // Ring card titles
    totalQuota: '总额度',
    limit5h: '5 小时限额',
    limitDaily: '日限额',
    limit7d: '7 天限额',
    limitWeekly: '周限额',
    limitMonthly: '月限额',
    // Detail rows
    remainingQuota: '剩余额度',
    expiresAt: '过期时间',
    todayExpires: '(今日到期)',
    daysLeft: '({days} 天)',
    usedQuota: '已用额度',
    resetNow: '即将重置',
    subscriptionType: '订阅类型',
    subscriptionExpires: '订阅到期',
    // Usage stat cells
    todayRequests: '今日请求',
    todayInputTokens: '今日输入',
    todayOutputTokens: '今日输出',
    todayTokens: '今日 Tokens',
    todayCacheCreation: '今日缓存创建',
    todayCacheRead: '今日缓存读取',
    todayCost: '今日费用',
    rpmTpm: 'RPM / TPM',
    totalRequests: '累计请求',
    totalInputTokens: '累计输入',
    totalOutputTokens: '累计输出',
    totalTokensLabel: '累计 Tokens',
    totalCacheCreation: '累计缓存创建',
    totalCacheRead: '累计缓存读取',
    totalCost: '累计费用',
    avgDuration: '平均耗时',
    // Messages
    enterApiKey: '请输入 API Key',
    querySuccess: '查询成功',
    queryFailed: '查询失败',
    queryFailedRetry: '查询失败，请稍后重试',
    noDailyUsage: '暂无按日用量数据',
  },

  // Setup Wizard
  setup: {
    title: 'Sub2API 安装向导',
    description: '配置您的 Sub2API 实例',
    database: {
      title: '数据库配置',
      description: '连接到您的 PostgreSQL 数据库',
      host: '主机',
      port: '端口',
      username: '用户名',
      password: '密码',
      databaseName: '数据库名称',
      sslMode: 'SSL 模式',
      passwordPlaceholder: '密码',
      ssl: {
        disable: '禁用',
        require: '要求',
        verifyCa: '验证 CA',
        verifyFull: '完全验证'
      }
    },
    redis: {
      title: 'Redis 配置',
      description: '连接到您的 Redis 服务器',
      host: '主机',
      port: '端口',
      username: '用户名（可选）',
      password: '密码（可选）',
      database: '数据库',
      usernamePlaceholder: '默认用户留空',
      passwordPlaceholder: '密码',
      enableTls: '启用 TLS',
      enableTlsHint: '连接 Redis 时使用 TLS（公共 CA 证书）'
    },
    admin: {
      title: '管理员账户',
      description: '创建您的管理员账户',
      email: '邮箱',
      password: '密码',
      confirmPassword: '确认密码',
      passwordPlaceholder: '至少 8 个字符',
      confirmPasswordPlaceholder: '确认密码',
      passwordMismatch: '密码不匹配'
    },
    ready: {
      title: '准备安装',
      description: '检查您的配置并完成安装',
      database: '数据库',
      redis: 'Redis',
      adminEmail: '管理员邮箱'
    },
    status: {
      testing: '测试中...',
      success: '连接成功',
      testConnection: '测试连接',
      installing: '安装中...',
      completeInstallation: '完成安装',
      completed: '安装完成！',
      redirecting: '正在跳转到登录页面...',
      restarting: '服务正在重启，请稍候...',
      timeout: '服务重启时间超出预期，请手动刷新页面。'
    }
  },

  // Common
}
