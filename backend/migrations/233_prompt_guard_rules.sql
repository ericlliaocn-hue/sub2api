-- Versioned high-confidence fast-path rules for synchronous prompt auditing.
-- Rules are intentionally action-plus-signal patterns; broad single-word
-- matches are avoided so ordinary safety discussions continue to Guard.

CREATE TABLE IF NOT EXISTS prompt_guard_rules (
    id           BIGSERIAL PRIMARY KEY,
    rule_key     VARCHAR(100) NOT NULL UNIQUE,
    name         VARCHAR(200) NOT NULL,
    category     VARCHAR(100) NOT NULL,
    severity     VARCHAR(16) NOT NULL,
    pattern_type VARCHAR(16) NOT NULL,
    pattern      TEXT NOT NULL,
    action       VARCHAR(16) NOT NULL DEFAULT 'block',
    priority     INT NOT NULL DEFAULT 100,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by   BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT chk_prompt_guard_rules_severity CHECK (severity IN ('medium', 'high', 'critical')),
    CONSTRAINT chk_prompt_guard_rules_pattern_type CHECK (pattern_type IN ('literal', 'regex')),
    CONSTRAINT chk_prompt_guard_rules_action CHECK (action IN ('block', 'flag')),
    CONSTRAINT chk_prompt_guard_rules_priority CHECK (priority > 0 AND priority <= 100000),
    CONSTRAINT chk_prompt_guard_rules_pattern_nonempty CHECK (length(btrim(pattern)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_prompt_guard_rules_enabled_priority
    ON prompt_guard_rules(enabled, priority, id);

INSERT INTO prompt_guard_rules
    (rule_key, name, category, severity, pattern_type, pattern, action, priority)
VALUES
    ('sexual-literary-generation', '色情文学生成', 'sexual_content_or_sexual_acts', 'critical', 'regex', E'(?is)(写|创作|生成|续写|扩写|润色|描写).{0,40}(色情|淫秽|情色|肉文|黄文|性描写|性交|性爱)', 'block', 10),
    ('minor-sexual-content', '未成年人色情内容', 'sexual_content_or_sexual_acts', 'critical', 'regex', E'(?is)(未成年|幼女|萝莉|儿童|学生).{0,24}(色情|性|裸|淫|性交|情色)', 'block', 5),
    ('credential-exfiltration', '凭据或密钥盗取', 'non_violent_illegal_acts', 'critical', 'regex', E'(?is)(窃取|盗取|抓取|获取|导出|dump|steal|exfiltrat).{0,60}(密码|口令|凭据|cookie|token|api[_ -]?key|密钥|账号)', 'block', 20),
    ('phishing-credential-collection', '钓鱼凭据收集', 'non_violent_illegal_acts', 'critical', 'regex', E'(?is)(钓鱼|仿冒|伪造登录|登录页).{0,60}(密码|凭据|cookie|token|账号)', 'block', 21),
    ('malware-exploit-development', '恶意软件或漏洞攻击', 'non_violent_illegal_acts', 'critical', 'regex', E'(?is)(编写|生成|部署|利用|攻击|入侵|绕过).{0,60}(木马|病毒|勒索|恶意软件|漏洞|exploit|payload|shellcode|提权)', 'block', 22),
    ('automated-bulk-scraping', '批量自动化爬取', 'non_violent_illegal_acts', 'high', 'regex', E'(?is)(批量|自动化|高并发|循环).{0,30}(爬取|抓取|采集|扒取).{0,50}(网站|接口|用户|邮箱|数据)', 'block', 30),
    ('privacy-doxxing', '隐私开盒与人肉', 'pii', 'critical', 'regex', E'(?is)(人肉|开盒|曝光|扒出|定位).{0,50}(住址|手机号|身份证|隐私|个人信息|真实身份)', 'block', 31),
    ('safety-evasion', '安全审计绕过', 'jailbreak', 'high', 'regex', E'(?is)(绕过|规避|隐藏|逃避).{0,40}(安全|审计|审核|检测|封禁|限制|策略)', 'block', 32),
    ('weapon-or-bio-construction', '武器或生化物制造', 'violent', 'critical', 'regex', E'(?is)(制造|组装|制作).{0,50}(炸弹|爆炸物|枪|生化|化学武器)', 'block', 40),
    ('fraud-financial-crime', '金融欺诈与洗钱', 'non_violent_illegal_acts', 'high', 'regex', E'(?is)(伪造|制作|骗取|洗钱|套现).{0,50}(发票|银行卡|支付|转账|金融|验证码)', 'block', 41)
ON CONFLICT (rule_key) DO NOTHING;
