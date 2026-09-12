<template>
  <HomeView v-if="hasHomeContent || compactHomeEnabled" />

  <div v-else class="oi-page" :class="{ 'is-latin': isLatin }">
    <!-- 首屏：整块朱红 -->
    <section class="oi-hero">
      <div class="oi-wrap">
        <header class="oi-top">
          <RouterLink to="/" class="oi-brand" aria-label="oioio home">
            <img :src="siteLogo" alt="" />
            <span>{{ siteName }}</span>
          </RouterLink>
          <nav class="oi-nav" aria-label="Primary navigation">
            <RouterLink to="/model-plaza">{{ t('home.v4.nav.models') }}</RouterLink>
            <RouterLink :to="memberPath('/creation')">{{ t('home.v4.nav.creation') }}</RouterLink>
            <RouterLink :to="memberPath('/purchase')">{{ t('home.v4.nav.pricing') }}</RouterLink>
            <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
          </nav>
          <div class="oi-tools">
            <LocaleSwitcher />
            <button type="button" class="oi-tool" :title="isDark ? t('home.switchToLight') : t('home.switchToDark')" :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')" @click="toggleTheme">
              <Icon v-if="isDark" name="sun" size="sm" />
              <Icon v-else name="moon" size="sm" />
            </button>
            <RouterLink :to="accountPath" class="oi-btn oi-btn-sm">{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}</RouterLink>
          </div>
        </header>

        <div class="oi-hero-grid">
          <p class="oi-meta">{{ t('home.v4.hero.meta') }}</p>
          <p class="oi-meta oi-meta-right"><code>{{ baseHost }}</code></p>
          <h1>{{ t('home.v4.hero.title') }}</h1>
          <p class="oi-accent">{{ t('home.v4.hero.accent') }}</p>
          <p class="oi-lede">{{ t('home.v4.hero.description') }}</p>
          <div class="oi-actions">
            <RouterLink to="/model-plaza" class="oi-btn oi-btn-fill">{{ t('home.v4.hero.primary') }}</RouterLink>
            <RouterLink :to="accountPath" class="oi-btn">{{ t('home.v4.hero.secondary') }}</RouterLink>
          </div>
        </div>

        <!-- o i o i o：进 → 出 -->
        <div class="oi-io" aria-hidden="true">
          <span class="oi-io-cap">{{ t('home.v4.hero.ioIn') }}</span>
          <div class="oi-io-row">
            <i class="oi-o"></i><i class="oi-i"></i><i class="oi-o"></i><i class="oi-i"></i><i class="oi-o"></i>
          </div>
          <span class="oi-io-cap">{{ t('home.v4.hero.ioOut') }}</span>
        </div>
      </div>
    </section>

    <!-- 跑马灯 -->
    <div class="oi-ticker" aria-hidden="true">
      <div class="oi-ticker-track">
        <span v-for="(item, index) in tickerItems" :key="index">{{ item }}</span>
      </div>
    </div>

    <main>
      <!-- 三件事 -->
      <section class="oi-three">
        <div class="oi-wrap">
          <p class="oi-label">{{ t('home.v4.three.label') }}</p>
          <div class="oi-three-grid">
            <div v-for="n in 3" :key="n">
              <b>{{ String(n).padStart(2, '0') }}</b>
              <h2>{{ t(`home.v4.three.n${n}Title`) }}</h2>
              <p>{{ t(`home.v4.three.n${n}Desc`) }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- 模型 -->
      <section class="oi-models">
        <div class="oi-wrap">
          <div class="oi-models-head">
            <p class="oi-label">{{ t('home.v4.models.label') }}</p>
            <h2>{{ t('home.v4.models.title') }}</h2>
            <p>{{ t('home.v4.models.note') }}</p>
          </div>
          <div class="oi-rows">
            <RouterLink v-for="model in models" :key="model.name" to="/model-plaza" class="oi-row">
              <PlatformIcon :platform="model.platform" size="md" />
              <strong>{{ model.name }}</strong>
              <code>{{ model.protocol }}</code>
              <span aria-hidden="true">→</span>
            </RouterLink>
          </div>
          <RouterLink to="/model-plaza" class="oi-more">{{ t('home.v4.models.more') }} <span aria-hidden="true">→</span></RouterLink>
        </div>
      </section>

      <!-- 接入（反色块） -->
      <section class="oi-code">
        <div class="oi-wrap oi-code-grid">
          <div class="oi-code-copy">
            <p class="oi-label">{{ t('home.v4.code.label') }}</p>
            <h2>{{ t('home.v4.code.title') }}<em>{{ t('home.v4.code.accent') }}</em></h2>
            <p>{{ t('home.v4.code.description') }}</p>
            <div class="oi-base">
              <span>{{ t('home.v4.code.baseLabel') }}</span>
              <code>{{ baseUrl }}</code>
              <button type="button" @click="copyBaseUrl">{{ copied ? t('home.v4.code.copied') : t('home.v4.code.copy') }}</button>
            </div>
          </div>
          <div class="oi-code-box">
            <div class="oi-code-tabs" role="tablist">
              <button v-for="tab in codeTabs" :key="tab.id" type="button" role="tab" :aria-selected="codeTab === tab.id" :class="{ 'is-active': codeTab === tab.id }" @click="codeTab = tab.id">{{ t(tab.label) }}</button>
            </div>
            <div class="oi-code-body">
              <div v-for="(line, index) in codeLines" :key="codeTab + index" class="oi-line" :class="line.tone">{{ line.text }}</div>
            </div>
          </div>
        </div>
      </section>

      <!-- 常见问题 -->
      <section class="oi-faq">
        <div class="oi-wrap">
          <p class="oi-label">{{ t('home.v4.faq.label') }}</p>
          <dl>
            <div v-for="n in 3" :key="n">
              <dt>{{ t(`home.v4.faq.q${n}`) }}</dt>
              <dd>{{ t(`home.v4.faq.a${n}`) }}</dd>
            </div>
          </dl>
        </div>
      </section>

      <!-- CTA -->
      <section class="oi-cta">
        <div class="oi-wrap">
          <div class="oi-cta-in">
            <h2>{{ t('home.v4.cta.title') }}</h2>
            <div class="oi-cta-side">
              <p>{{ t('home.v4.cta.description') }}</p>
              <div class="oi-actions">
                <RouterLink to="/model-plaza" class="oi-btn oi-btn-fill">{{ t('home.v4.cta.action') }}</RouterLink>
                <RouterLink :to="accountPath" class="oi-btn">{{ t('home.v4.hero.secondary') }}</RouterLink>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="oi-footer">
      <div class="oi-wrap">
        <span>© {{ currentYear }} {{ siteName }} · {{ t('home.v4.footer') }}</span>
        <nav>
          <RouterLink to="/model-plaza">{{ t('home.v4.nav.models') }}</RouterLink>
          <RouterLink :to="memberPath('/creation')">{{ t('home.v4.nav.creation') }}</RouterLink>
          <RouterLink to="/key-usage">{{ t('home.v4.footerUsage') }}</RouterLink>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
        </nav>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import HomeView from '@/views/HomeView.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

type CodeTab = 'curl' | 'python' | 'node'
interface CodeLine { text: string; tone?: string }

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const isDark = ref(document.documentElement.classList.contains('dark'))
const copied = ref(false)
const codeTab = ref<CodeTab>('curl')
let copiedTimer: ReturnType<typeof setTimeout> | undefined

const models = [
  { name: 'Claude', platform: 'anthropic' as const, protocol: 'Anthropic Messages · Chat Completions' },
  { name: 'GPT', platform: 'openai' as const, protocol: 'Responses · Chat Completions' },
  { name: 'Gemini', platform: 'gemini' as const, protocol: 'generateContent · Chat Completions' },
  { name: 'Grok', platform: 'grok' as const, protocol: 'Chat Completions' },
]
const tickerBase = ['Claude', 'GPT', 'Gemini', 'Grok', 'Chat Completions', 'Responses API', 'Anthropic Messages', 'generateContent', 'Codex CLI', 'Claude Code', 'OpenAI SDK', '/v1']
const tickerItems = [...tickerBase, ...tickerBase, ...tickerBase, ...tickerBase]
const codeTabs: { id: CodeTab; label: string }[] = [
  { id: 'curl', label: 'home.v4.code.tabCurl' },
  { id: 'python', label: 'home.v4.code.tabPython' },
  { id: 'node', label: 'home.v4.code.tabNode' },
]

const isLatin = computed(() => !String(locale.value).toLowerCase().startsWith('zh'))
const siteName = computed(() => {
  const configured = appStore.cachedPublicSettings?.site_name || appStore.siteName || ''
  const normalized = configured.trim().toLowerCase()
  return configured.trim() && normalized !== 'anytoken' && normalized !== 'sub2api' ? configured : 'oioio'
})
const siteLogo = computed(() => {
  const configured = sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true })
  return configured && !configured.toLowerCase().includes('anytoken') ? configured : '/oioio-logo.svg'
})
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const hasHomeContent = computed(() => (appStore.cachedPublicSettings?.home_content || '').trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const accountPath = computed(() => (isAuthenticated.value ? dashboardPath.value : '/login'))
const currentYear = computed(() => new Date().getFullYear())
const API_BASE_HOST = 'api.oioio.chat/v1'
const baseUrl = computed(() => `https://${API_BASE_HOST}`)
const baseHost = computed(() => API_BASE_HOST)

const codeLines = computed<CodeLine[]>(() => {
  const prompt = t('home.v4.code.prompt')
  if (codeTab.value === 'python') {
    return [
      { text: 'from openai import OpenAI', tone: 'is-dim' },
      { text: '' },
      { text: 'client = OpenAI(' },
      { text: `    base_url="${baseUrl.value}",`, tone: 'is-hi' },
      { text: '    api_key="sk-oioio-••••••••",' },
      { text: ')' },
      { text: 'reply = client.chat.completions.create(' },
      { text: '    model="claude-sonnet",' },
      { text: `    messages=[{"role": "user", "content": "${prompt}"}],` },
      { text: ')' },
      { text: 'print(reply.choices[0].message.content)' },
    ]
  }
  if (codeTab.value === 'node') {
    return [
      { text: 'import OpenAI from "openai"', tone: 'is-dim' },
      { text: '' },
      { text: 'const client = new OpenAI({' },
      { text: `  baseURL: "${baseUrl.value}",`, tone: 'is-hi' },
      { text: '  apiKey: process.env.OIOIO_API_KEY,' },
      { text: '})' },
      { text: 'const reply = await client.chat.completions.create({' },
      { text: '  model: "claude-sonnet",' },
      { text: `  messages: [{ role: "user", content: "${prompt}" }],` },
      { text: '})' },
      { text: 'console.log(reply.choices[0].message.content)' },
    ]
  }
  return [
    { text: `curl ${baseUrl.value}/chat/completions \\`, tone: 'is-hi' },
    { text: '  -H "Authorization: Bearer $OIOIO_API_KEY" \\' },
    { text: '  -H "Content-Type: application/json" \\' },
    { text: '  -d \'{' },
    { text: '    "model": "claude-sonnet",' },
    { text: `    "messages": [{ "role": "user", "content": "${prompt}" }]` },
    { text: '  }\'' },
  ]
})

function memberPath(path: string) { return isAuthenticated.value ? path : '/login' }
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}
async function copyBaseUrl() {
  try {
    await navigator.clipboard.writeText(baseUrl.value)
    copied.value = true
    if (copiedTimer) clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => { copied.value = false }, 1600)
  } catch {
    copied.value = false
  }
}
onMounted(() => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
})
onBeforeUnmount(() => { if (copiedTimer) clearTimeout(copiedTimer) })
</script>

<style scoped>
/* ---- 纸 / 墨 / 朱红 ---- */
.oi-page {
  --paper: #f3efe6;
  --ink: #121212;
  --muted: #6d6a63;
  --line: rgba(18, 18, 18, .16);
  --red: #ff3d00;
  --red-ink: #121212;
  --block: #121212;
  --block-ink: #f3efe6;
  --block-muted: #9b978e;
  --block-line: rgba(243, 239, 230, .18);
  --sans: "Helvetica Neue", -apple-system, BlinkMacSystemFont, "PingFang SC", "Hiragino Sans GB", "Noto Sans SC", "Microsoft YaHei", Arial, sans-serif;
  --serif: "Songti SC", "STSong", "Noto Serif SC", "Source Han Serif SC", Georgia, "Times New Roman", serif;
  --mono: "SF Mono", ui-monospace, Menlo, Consolas, monospace;
  --gutter: 48px;
  min-height: 100vh;
  background: var(--paper);
  color: var(--ink);
  font-family: var(--sans);
  font-size: 15px;
  line-height: 1.7;
  -webkit-font-smoothing: antialiased;
}
.dark .oi-page {
  --paper: #0f0f0f;
  --ink: #f1ede4;
  --muted: #9a968e;
  --line: rgba(241, 237, 228, .16);
  --block: #f1ede4;
  --block-ink: #121212;
  --block-muted: #6d6a63;
  --block-line: rgba(18, 18, 18, .16);
}
:where(.oi-page a) { color: inherit; text-decoration: none; }
.oi-page :is(a, button):focus-visible { outline: 2px solid currentColor; outline-offset: 3px; }
.oi-page svg { flex: none; }
.oi-wrap { max-width: 1280px; margin: 0 auto; padding: 0 var(--gutter); }

/* ---- 通用 ---- */
.oi-label { display: flex; align-items: center; gap: 12px; margin: 0; color: var(--muted); font-family: var(--mono); font-size: 12px; letter-spacing: .06em; text-transform: uppercase; }
.oi-label::after { content: ''; flex: 1; height: 1px; background: var(--line); }
.oi-btn { display: inline-flex; align-items: center; justify-content: center; min-height: 48px; padding: 0 22px; border: 1.5px solid currentColor; border-radius: 999px; font-size: 14.5px; font-weight: 600; white-space: nowrap; cursor: pointer; transition: background .18s ease, color .18s ease, border-color .18s ease, transform .18s ease; }
.oi-btn:hover { transform: translateY(-1px); }
.oi-btn-sm { min-height: 36px; padding: 0 16px; font-size: 13px; }
.oi-btn-fill { background: var(--ink); border-color: var(--ink); color: var(--paper); }
.oi-btn-fill:hover { background: transparent; color: var(--ink); }
.oi-hero .oi-btn-fill { background: var(--red-ink); border-color: var(--red-ink); color: var(--red); }
.oi-hero .oi-btn-fill:hover { background: transparent; color: var(--red-ink); }
.oi-hero .oi-btn:not(.oi-btn-fill):hover { background: var(--red-ink); color: var(--red); }
.oi-btn:not(.oi-btn-fill):hover { background: var(--ink); color: var(--paper); }
.oi-actions { display: flex; flex-wrap: wrap; gap: 12px; }

/* ---- 首屏 ---- */
.oi-hero { background: var(--red); color: var(--red-ink); }
.oi-top { display: flex; align-items: center; gap: 24px; padding: 20px 0; }
.oi-brand { display: inline-flex; align-items: center; gap: 10px; font-size: 18px; font-weight: 700; letter-spacing: -.01em; }
.oi-brand img { width: 30px; height: 30px; border-radius: 9px; }
.oi-nav { display: flex; gap: 24px; margin-left: 24px; font-size: 14px; font-weight: 500; }
.oi-nav a { border-bottom: 1.5px solid transparent; transition: border-color .15s ease; }
.oi-nav a:hover { border-bottom-color: currentColor; }
.oi-tools { display: flex; align-items: center; gap: 8px; margin-left: auto; }
.oi-tool { display: grid; place-items: center; width: 36px; height: 36px; border: 1.5px solid transparent; border-radius: 50%; background: transparent; color: inherit; cursor: pointer; transition: border-color .15s ease; }
.oi-tool:hover { border-color: currentColor; }
.oi-tools :deep(.relative > button) { color: inherit; font-weight: 600; }
.oi-tools :deep(.relative > button:hover) { background: rgba(18, 18, 18, .1); }
.oi-tools :deep(.relative > button svg) { color: inherit; opacity: .7; }

.oi-hero-grid { display: grid; grid-template-columns: minmax(0, 1fr) auto; padding: 72px 0 0; }
.oi-meta { grid-column: 1; margin: 0; font-family: var(--mono); font-size: 12.5px; letter-spacing: .04em; }
.oi-meta-right { grid-column: 2; text-align: right; }
.oi-meta code { font-family: inherit; }
.oi-hero h1 { grid-column: 1 / -1; margin: 28px 0 0; font-size: clamp(56px, 8.6vw, 128px); font-weight: 700; line-height: .98; letter-spacing: -.04em; white-space: pre-line; }
.is-latin .oi-hero h1 { letter-spacing: -.045em; }
.oi-accent { grid-column: 1 / -1; margin: 22px 0 0; font-family: var(--serif); font-size: clamp(24px, 3vw, 40px); font-weight: 400; line-height: 1.25; }
.is-latin .oi-accent { font-style: italic; }
.oi-lede { grid-column: 1; max-width: 560px; margin: 32px 0 0; font-size: 17px; line-height: 1.7; }
.oi-hero .oi-actions { grid-column: 2; align-self: end; justify-content: flex-end; margin-top: 32px; }

/* o i o i o */
.oi-io { --u: clamp(64px, 11vw, 160px); display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: end; gap: 32px; padding: 72px 0 56px; }
.oi-io-cap { padding-bottom: 4px; font-family: var(--mono); font-size: 12.5px; letter-spacing: .1em; text-transform: uppercase; }
.oi-io-row { display: grid; grid-template-columns: var(--u) calc(var(--u) * .3) var(--u) calc(var(--u) * .3) var(--u); justify-content: space-between; align-items: end; }
.oi-io-row i { position: relative; display: block; }
.oi-o { height: var(--u); border: 3px solid var(--red-ink); border-radius: 50%; }
.oi-o::after { content: ''; position: absolute; inset: 18%; border-radius: 50%; background: var(--red-ink); transform: scale(0); animation: oi-pulse 3s ease-in-out infinite; }
.oi-i { height: calc(var(--u) * .62); background: var(--red-ink); border-radius: 2px; }
.oi-i::before { content: ''; position: absolute; left: 0; right: 0; bottom: calc(100% + var(--u) * .1); aspect-ratio: 1; border-radius: 50%; background: var(--red-ink); }
.oi-i::after { content: ''; position: absolute; inset: 0; border-radius: 2px; background: var(--red); opacity: 0; animation: oi-blink 3s ease-in-out infinite; }
.oi-io-row i:nth-child(1)::after { animation-delay: 0s; }
.oi-io-row i:nth-child(2)::after { animation-delay: .3s; }
.oi-io-row i:nth-child(3)::after { animation-delay: .6s; }
.oi-io-row i:nth-child(4)::after { animation-delay: .9s; }
.oi-io-row i:nth-child(5)::after { animation-delay: 1.2s; }
@keyframes oi-pulse { 0%, 55%, 100% { transform: scale(0); } 12%, 32% { transform: scale(1); } }
@keyframes oi-blink { 0%, 55%, 100% { opacity: 0; } 12%, 32% { opacity: .4; } }

/* ---- 跑马灯 ---- */
.oi-ticker { overflow: hidden; padding: 14px 0; background: var(--block); color: var(--block-ink); }
.oi-ticker-track { display: flex; width: max-content; animation: oi-marquee 48s linear infinite; }
.oi-ticker:hover .oi-ticker-track { animation-play-state: paused; }
.oi-ticker-track span { display: inline-flex; align-items: center; padding: 0 28px; font-family: var(--mono); font-size: 13px; letter-spacing: .04em; white-space: nowrap; }
.oi-ticker-track span::after { content: ''; width: 6px; height: 6px; margin-left: 56px; border-radius: 50%; background: var(--red); }
@keyframes oi-marquee { to { transform: translateX(-50%); } }

/* ---- 三件事 ---- */
.oi-three { padding: 96px 0 88px; }
.oi-three-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 48px; margin-top: 48px; }
.oi-three-grid b { display: block; color: transparent; -webkit-text-stroke: 1px var(--ink); font-family: var(--serif); font-size: clamp(88px, 9vw, 136px); font-weight: 400; line-height: .9; letter-spacing: -.04em; }
.oi-three-grid h2 { margin: 28px 0 0; font-size: 28px; font-weight: 700; letter-spacing: -.02em; line-height: 1.2; }
.oi-three-grid p { max-width: 340px; margin: 14px 0 0; color: var(--muted); font-size: 14.5px; line-height: 1.75; }

/* ---- 模型行 ---- */
.oi-models { padding: 0 0 96px; }
.oi-models-head { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); align-items: end; gap: 24px 48px; }
.oi-models-head .oi-label { grid-column: 1 / -1; }
.oi-models-head h2 { margin: 24px 0 0; font-size: clamp(36px, 4vw, 56px); font-weight: 700; letter-spacing: -.03em; line-height: 1.05; }
.oi-models-head > p:not(.oi-label) { max-width: 420px; margin: 0; padding-bottom: 6px; color: var(--muted); font-size: 14.5px; }
.oi-rows { margin-top: 40px; border-top: 1.5px solid var(--ink); }
.oi-row { display: grid; grid-template-columns: 40px minmax(0, 1fr) auto 32px; align-items: center; gap: 24px; padding: 26px 0; border-bottom: 1px solid var(--line); transition: background .18s ease, color .18s ease, padding .18s ease; }
.oi-row svg { width: 28px; height: 28px; }
.oi-row strong { font-size: clamp(30px, 3vw, 44px); font-weight: 700; letter-spacing: -.03em; line-height: 1; }
.oi-row code { color: var(--muted); font-family: var(--mono); font-size: 13px; text-align: right; transition: color .18s ease; }
.oi-row span { font-size: 24px; line-height: 1; text-align: right; transition: transform .18s ease; }
.oi-row:hover { background: var(--red); color: var(--red-ink); padding-left: 16px; padding-right: 16px; }
.oi-row:hover code { color: var(--red-ink); }
.oi-row:hover span { transform: translateX(6px); }
.oi-more { display: inline-flex; align-items: center; gap: 8px; margin-top: 28px; padding-bottom: 2px; border-bottom: 1.5px solid currentColor; font-size: 15px; font-weight: 600; transition: color .15s ease; }
.oi-more:hover { color: var(--red); }

/* ---- 接入（反色块） ---- */
.oi-code { padding: 96px 0; background: var(--block); color: var(--block-ink); }
.oi-code .oi-label { color: var(--block-muted); }
.oi-code .oi-label::after { background: var(--block-line); }
.oi-code-grid { display: grid; grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); gap: 64px; align-items: start; }
.oi-code-copy h2 { margin: 28px 0 0; font-size: clamp(34px, 3.4vw, 48px); font-weight: 700; letter-spacing: -.03em; line-height: 1.1; }
.oi-code-copy h2 em { display: block; font-family: var(--serif); font-style: normal; font-weight: 400; color: var(--red); }
.is-latin .oi-code-copy h2 em { font-style: italic; }
.oi-code-copy > p:not(.oi-label) { max-width: 420px; margin: 20px 0 0; color: var(--block-muted); font-size: 15px; line-height: 1.75; }
.oi-base { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 4px 16px; margin-top: 40px; padding: 16px 18px; border: 1px solid var(--block-line); border-radius: 12px; }
.oi-base span { grid-column: 1; color: var(--block-muted); font-family: var(--mono); font-size: 11px; letter-spacing: .08em; text-transform: uppercase; }
.oi-base code { grid-column: 1; overflow: hidden; font-family: var(--mono); font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.oi-base button { grid-column: 2; grid-row: 1 / 3; padding: 8px 14px; border: 1.5px solid currentColor; border-radius: 999px; background: transparent; color: inherit; font-size: 12.5px; font-weight: 600; cursor: pointer; transition: background .15s ease, color .15s ease; }
.oi-base button:hover { background: var(--block-ink); color: var(--block); }
.oi-code-box { border: 1px solid var(--block-line); border-radius: 14px; overflow: hidden; }
.oi-code-tabs { display: flex; gap: 4px; padding: 10px 10px 0; border-bottom: 1px solid var(--block-line); }
.oi-code-tabs button { padding: 8px 16px 12px; border: 0; border-bottom: 2px solid transparent; margin-bottom: -1px; background: transparent; color: var(--block-muted); font-family: var(--mono); font-size: 13px; cursor: pointer; transition: color .15s ease, border-color .15s ease; }
.oi-code-tabs button:hover { color: var(--block-ink); }
.oi-code-tabs button.is-active { color: var(--block-ink); border-bottom-color: var(--red); }
.oi-code-body { min-height: 282px; padding: 22px 22px 20px; overflow-x: auto; }
.oi-line { color: var(--block-ink); opacity: .82; font-family: var(--mono); font-size: 13px; line-height: 1.75; white-space: pre; }
.oi-line.is-hi { opacity: 1; color: var(--red); }
.oi-line.is-dim { opacity: .5; }

/* ---- 常见问题 ---- */
.oi-faq { padding: 96px 0 80px; }
.oi-faq dl { display: grid; gap: 0; margin: 32px 0 0; }
.oi-faq dl > div { display: grid; grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); gap: 16px 64px; padding: 28px 0; border-bottom: 1px solid var(--line); }
.oi-faq dl > div:first-child { border-top: 1.5px solid var(--ink); }
.oi-faq dt { font-size: 20px; font-weight: 700; letter-spacing: -.01em; line-height: 1.35; }
.oi-faq dd { margin: 0; max-width: 620px; color: var(--muted); font-size: 15px; line-height: 1.8; }

/* ---- CTA ---- */
.oi-cta { padding: 40px 0 112px; }
.oi-cta-in { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: end; gap: 32px 64px; padding-top: 48px; border-top: 1.5px solid var(--ink); }
.oi-cta h2 { margin: 0; font-family: var(--serif); font-size: clamp(64px, 10vw, 152px); font-weight: 400; line-height: .95; letter-spacing: -.04em; }
.is-latin .oi-cta h2 { font-style: italic; }
.oi-cta-side { display: grid; gap: 20px; justify-items: end; text-align: right; }
.oi-cta-side p { max-width: 360px; margin: 0; color: var(--muted); font-size: 15px; }

/* ---- 页脚 ---- */
.oi-footer { padding: 24px 0 32px; border-top: 1px solid var(--line); color: var(--muted); font-size: 12.5px; }
.oi-footer .oi-wrap { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.oi-footer nav { display: flex; flex-wrap: wrap; gap: 20px; }
.oi-footer a:hover { color: var(--ink); }

@media (prefers-reduced-motion: reduce) {
  .oi-page *, .oi-page *::before, .oi-page *::after { animation: none !important; transition: none !important; }
  .oi-o::after { transform: scale(1); }
}
@media (max-width: 1024px) {
  .oi-page { --gutter: 32px; }
  .oi-hero-grid { grid-template-columns: minmax(0, 1fr); padding-top: 56px; }
  .oi-meta-right { display: none; }
  .oi-hero .oi-actions { grid-column: 1; justify-content: flex-start; }
  .oi-io { padding: 56px 0 44px; }
  .oi-three { padding: 72px 0; }
  .oi-three-grid { gap: 40px 32px; }
  .oi-models-head { grid-template-columns: minmax(0, 1fr); }
  .oi-models-head > p:not(.oi-label) { padding-bottom: 0; }
  .oi-code-grid { grid-template-columns: minmax(0, 1fr); gap: 40px; }
  .oi-code { padding: 72px 0; }
  .oi-faq dl > div { grid-template-columns: minmax(0, 1fr); gap: 10px; }
  .oi-cta-in { grid-template-columns: minmax(0, 1fr); }
  .oi-cta-side { justify-items: start; text-align: left; }
}
@media (max-width: 720px) {
  .oi-page { --gutter: 20px; font-size: 14px; }
  .oi-top { gap: 10px; padding: 14px 0; }
  .oi-nav { display: none; }
  .oi-tools { gap: 4px; }
  .oi-hero-grid { padding-top: 40px; }
  .oi-hero h1 { margin-top: 20px; font-size: clamp(44px, 13vw, 60px); }
  .oi-accent { margin-top: 16px; font-size: 20px; }
  .oi-lede { margin-top: 22px; font-size: 15.5px; }
  .oi-hero .oi-actions { margin-top: 26px; }
  .oi-btn { min-height: 44px; padding: 0 18px; font-size: 14px; }
  .oi-io { --u: 52px; gap: 14px; padding: 44px 0 36px; }
  .oi-o { border-width: 2px; }
  .oi-ticker { padding: 11px 0; }
  .oi-ticker-track span { padding: 0 16px; font-size: 12px; }
  .oi-ticker-track span::after { margin-left: 32px; }
  .oi-three { padding: 56px 0; }
  .oi-three-grid { grid-template-columns: minmax(0, 1fr); gap: 36px; margin-top: 32px; }
  .oi-three-grid b { font-size: 72px; }
  .oi-three-grid h2 { margin-top: 14px; font-size: 24px; }
  .oi-models { padding-bottom: 64px; }
  .oi-rows { margin-top: 28px; }
  .oi-row { grid-template-columns: 28px minmax(0, 1fr) 24px; grid-template-rows: auto auto; gap: 6px 14px; padding: 18px 0; }
  .oi-row:hover { padding-left: 10px; padding-right: 10px; }
  .oi-row svg { width: 22px; height: 22px; }
  .oi-row strong { font-size: 28px; }
  .oi-row code { grid-column: 2; grid-row: 2; text-align: left; font-size: 12px; }
  .oi-row span { grid-column: 3; grid-row: 1; font-size: 20px; }
  .oi-code { padding: 56px 0; }
  .oi-base { margin-top: 28px; }
  .oi-code-body { min-height: 0; padding: 16px; }
  .oi-line { font-size: 12px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .oi-faq { padding: 56px 0 48px; }
  .oi-faq dt { font-size: 17px; }
  .oi-cta { padding: 16px 0 72px; }
  .oi-cta-in { padding-top: 32px; }
  .oi-footer .oi-wrap { flex-direction: column; align-items: flex-start; }
}
</style>
