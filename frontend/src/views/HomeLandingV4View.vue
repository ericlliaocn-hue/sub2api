<template>
  <HomeView v-if="hasHomeContent || compactHomeEnabled" />

  <div v-else class="v4-page min-h-screen bg-[#f5f5f3] text-[#1d1d1f] dark:bg-[#0b0c0d] dark:text-[#f5f5f7]">
    <header class="relative z-30 px-4 pt-4 sm:px-6 lg:px-10">
      <nav class="v4-nav mx-auto flex max-w-[1280px] items-center justify-between gap-3 px-3 py-2.5 sm:px-4">
        <RouterLink to="/home" class="flex min-w-0 items-center gap-2.5" aria-label="Home">
          <span class="v4-logo"><img :src="siteLogo || '/logo.svg'" alt="" class="h-full w-full object-contain" /></span>
          <span class="truncate text-[13px] font-semibold">{{ siteName }}</span>
        </RouterLink>

        <div class="hidden items-center gap-7 text-[13px] text-black/60 dark:text-white/60 md:flex">
          <RouterLink to="/model-plaza" class="v4-nav-link">{{ t('home.v4.nav.models') }}</RouterLink>
          <RouterLink :to="memberPath('/creation')" class="v4-nav-link">{{ t('home.v4.nav.creation') }}</RouterLink>
          <RouterLink :to="memberPath('/purchase')" class="v4-nav-link">{{ t('home.v4.nav.pricing') }}</RouterLink>
        </div>

        <div class="flex items-center gap-0.5 sm:gap-1">
          <LocaleSwitcher />
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="v4-icon-button" :title="t('home.viewDocs')"><Icon name="book" size="sm" /></a>
          <button type="button" class="v4-icon-button" :title="isDark ? t('home.switchToLight') : t('home.switchToDark')" @click="toggleTheme">
            <Icon v-if="isDark" name="sun" size="sm" /><Icon v-else name="moon" size="sm" />
          </button>
          <RouterLink :to="isAuthenticated ? dashboardPath : '/login'" class="v4-account-link">{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}<Icon name="arrowRight" size="xs" /></RouterLink>
        </div>
      </nav>
    </header>

    <main class="relative z-10">
      <section class="v4-hero mx-auto max-w-[1280px] px-5 pb-16 pt-14 sm:px-8 lg:px-10 lg:pb-20 lg:pt-20">
        <div class="grid items-center gap-12 lg:grid-cols-[.76fr_1.24fr] lg:gap-16">
          <div class="v4-entry max-w-xl text-center lg:text-left">
            <div class="v4-eyebrow"><span></span>{{ t('home.v4.hero.eyebrow') }}</div>
            <h1 class="v4-title mt-5">{{ t('home.v4.hero.title') }}<em>{{ t('home.v4.hero.accent') }}</em></h1>
            <p class="mx-auto mt-5 max-w-xl text-[15px] leading-7 text-black/58 dark:text-white/58 lg:mx-0">{{ t('home.v4.hero.description') }}</p>
            <div class="mt-7 flex flex-wrap items-center justify-center gap-3 lg:justify-start">
              <RouterLink to="/model-plaza" class="v4-primary-button">{{ t('home.v4.hero.primary') }}<Icon name="arrowRight" size="sm" /></RouterLink>
              <RouterLink :to="memberPath('/creation')" class="v4-secondary-button">{{ t('home.v4.hero.secondary') }}</RouterLink>
            </div>
          </div>

          <div class="v4-product-window w-full text-left">
            <div class="v4-window-toolbar">
              <div class="flex items-center gap-1.5"><i class="v4-dot v4-dot-red"></i><i class="v4-dot v4-dot-yellow"></i><i class="v4-dot v4-dot-green"></i></div>
              <span>{{ t('home.v4.surface.title') }}</span>
              <span class="v4-live-indicator"><b></b>{{ t('home.v4.surface.live') }}</span>
            </div>

            <div class="grid gap-0 lg:grid-cols-[208px_1fr]">
              <aside class="v4-window-sidebar hidden border-r border-black/8 p-5 dark:border-white/8 lg:block">
                <p class="v4-sidebar-label">{{ t('home.v4.surface.workspace') }}</p>
                <div class="mt-5 space-y-1.5">
                  <button type="button" class="v4-sidebar-item" :class="{ 'v4-sidebar-item-active': activeSurface === 'models' }" @click="activeSurface = 'models'"><Icon name="grid" size="sm" />{{ t('home.v4.surface.models') }}</button>
                  <button type="button" class="v4-sidebar-item" :class="{ 'v4-sidebar-item-active': activeSurface === 'creation' }" @click="activeSurface = 'creation'"><Icon name="sparkles" size="sm" />{{ t('home.v4.surface.creation') }}</button>
                  <button type="button" class="v4-sidebar-item" :class="{ 'v4-sidebar-item-active': activeSurface === 'usage' }" @click="activeSurface = 'usage'"><Icon name="chartBar" size="sm" />{{ t('home.v4.surface.usage') }}</button>
                </div>
                <div class="v4-sidebar-bottom"><span class="v4-avatar">{{ userInitial || 'AI' }}</span><span>{{ t('home.v4.surface.personal') }}</span><Icon name="chevronRight" size="xs" /></div>
              </aside>

              <div class="min-w-0 p-5 sm:p-7">
                <div class="flex flex-wrap items-start justify-between gap-4">
                  <div><p class="v4-overline">{{ t('home.v4.surface.greeting') }}</p><h2 class="mt-2 text-xl font-semibold text-[#1d1d1f] dark:text-white sm:text-2xl">{{ activeSurfaceTitle }}</h2></div>
                  <span class="v4-surface-chip"><b></b>{{ t('home.v4.surface.ready') }}</span>
                </div>

                <div class="v4-mobile-tabs mt-6 lg:hidden">
                  <button type="button" :class="{ 'v4-mobile-tab-active': activeSurface === 'models' }" @click="activeSurface = 'models'">{{ t('home.v4.surface.models') }}</button>
                  <button type="button" :class="{ 'v4-mobile-tab-active': activeSurface === 'creation' }" @click="activeSurface = 'creation'">{{ t('home.v4.surface.creation') }}</button>
                  <button type="button" :class="{ 'v4-mobile-tab-active': activeSurface === 'usage' }" @click="activeSurface = 'usage'">{{ t('home.v4.surface.usage') }}</button>
                </div>

                <div :key="activeSurface" class="v4-surface-content mt-7">
                  <template v-if="activeSurface === 'models'">
                    <div class="grid gap-3 sm:grid-cols-3">
                      <RouterLink to="/model-plaza" class="v4-model-card v4-model-card-active"><span class="v4-model-symbol v4-symbol-orange">C</span><strong>Claude</strong><small>{{ t('home.v4.surface.modelAvailable') }}</small><span class="v4-model-price">{{ t('home.v4.surface.modelFast') }}</span></RouterLink>
                      <RouterLink to="/model-plaza" class="v4-model-card"><span class="v4-model-symbol v4-symbol-green">G</span><strong>GPT</strong><small>{{ t('home.v4.surface.modelAvailable') }}</small><span class="v4-model-price">{{ t('home.v4.surface.modelPopular') }}</span></RouterLink>
                      <RouterLink to="/model-plaza" class="v4-model-card"><span class="v4-model-symbol v4-symbol-blue">G</span><strong>Gemini</strong><small>{{ t('home.v4.surface.modelAvailable') }}</small><span class="v4-model-price">{{ t('home.v4.surface.modelVisual') }}</span></RouterLink>
                    </div>
                    <div class="v4-surface-footer"><span>{{ t('home.v4.surface.modelFooter') }}</span><RouterLink to="/model-plaza">{{ t('home.v4.surface.openPlaza') }}<Icon name="arrowRight" size="xs" /></RouterLink></div>
                  </template>
                  <template v-else-if="activeSurface === 'creation'">
                    <div class="v4-creation-preview">
                      <div class="v4-creation-grid" aria-hidden="true"><i></i><i></i><i></i></div>
                      <span class="v4-creation-play"><Icon name="sparkles" size="sm" /></span>
                      <div><p>{{ t('home.v4.surface.creationLabel') }}</p><strong>{{ t('home.v4.surface.creationHeadline') }}</strong></div>
                    </div>
                    <div class="v4-surface-footer"><span>{{ t('home.v4.surface.creationFooter') }}</span><RouterLink :to="memberPath('/creation')">{{ t('home.v4.surface.openCreation') }}<Icon name="arrowRight" size="xs" /></RouterLink></div>
                  </template>
                  <template v-else>
                    <div class="v4-usage-preview"><div><span>{{ t('home.v4.surface.usageBalance') }}</span><strong>¥ 128.40</strong></div><div class="v4-usage-bars"><i style="height: 38%"></i><i style="height: 60%"></i><i style="height: 46%"></i><i style="height: 82%"></i><i style="height: 56%"></i><i style="height: 72%"></i><i style="height: 90%"></i></div></div>
                    <div class="v4-surface-footer"><span>{{ t('home.v4.surface.usageFooter') }}</span><RouterLink :to="memberPath('/usage')">{{ t('home.v4.surface.openUsage') }}<Icon name="arrowRight" size="xs" /></RouterLink></div>
                  </template>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="v4-proof-strip mt-14">
          <div><Icon name="key" size="sm" /><span><strong>{{ t('home.v4.hero.proof1Title') }}</strong><small>{{ t('home.v4.hero.proof1Desc') }}</small></span></div>
          <div><Icon name="swap" size="sm" /><span><strong>{{ t('home.v4.hero.proof2Title') }}</strong><small>{{ t('home.v4.hero.proof2Desc') }}</small></span></div>
          <div><Icon name="chartBar" size="sm" /><span><strong>{{ t('home.v4.hero.proof3Title') }}</strong><small>{{ t('home.v4.hero.proof3Desc') }}</small></span></div>
        </div>
      </section>

      <section id="api-access" class="v4-band border-y border-black/8 dark:border-white/8">
        <div class="mx-auto grid max-w-[1280px] gap-10 px-5 py-16 sm:px-8 lg:grid-cols-[.72fr_1.28fr] lg:gap-20 lg:px-10 lg:py-20">
          <div>
            <p class="v4-section-label">01 / {{ t('home.v4.access.label') }}</p>
            <h2 class="v4-section-title mt-4">{{ t('home.v4.access.title') }}</h2>
            <p class="mt-5 max-w-md text-sm leading-7 text-black/55 dark:text-white/55">{{ t('home.v4.access.description') }}</p>
          </div>

          <div class="v4-flow" aria-label="AI request flow">
            <div class="v4-flow-column">
              <p>{{ t('home.v4.access.clientsLabel') }}</p>
              <div class="v4-flow-list">
                <span><Icon name="terminal" size="sm" />Codex CLI</span>
                <span><Icon name="chat" size="sm" />Claude Code</span>
                <span><Icon name="cube" size="sm" />OpenAI SDK</span>
                <span><Icon name="sparkles" size="sm" />{{ t('home.v4.nav.creation') }}</span>
              </div>
            </div>

            <div class="v4-flow-arrow"><span></span><em>HTTPS</em><Icon name="arrowRight" size="sm" /></div>

            <div class="v4-gateway">
              <div class="v4-gateway-mark"><Icon name="server" size="md" /></div>
              <p>{{ t('home.v4.access.gatewayLabel') }}</p>
              <strong>{{ t('home.v4.access.gatewayTitle') }}</strong>
              <small>{{ t('home.v4.access.gatewayDesc') }}</small>
              <ul>
                <li><Icon name="check" size="xs" />{{ t('home.v4.access.gatewayKey') }}</li>
                <li><Icon name="check" size="xs" />{{ t('home.v4.access.gatewayRoute') }}</li>
                <li><Icon name="check" size="xs" />{{ t('home.v4.access.gatewayBilling') }}</li>
              </ul>
            </div>

            <div class="v4-flow-arrow"><span></span><em>API</em><Icon name="arrowRight" size="sm" /></div>

            <div class="v4-flow-column">
              <p>{{ t('home.v4.access.modelsLabel') }}</p>
              <div class="v4-flow-list v4-flow-models">
                <span><b class="v4-provider-dot v4-provider-orange"></b>Claude</span>
                <span><b class="v4-provider-dot v4-provider-green"></b>GPT</span>
                <span><b class="v4-provider-dot v4-provider-blue"></b>Gemini</span>
                <span><b class="v4-provider-dot v4-provider-gray"></b>{{ t('home.v4.access.moreModels') }}</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="mx-auto max-w-[1280px] px-5 py-16 sm:px-8 lg:px-10 lg:py-20">
        <div class="max-w-2xl">
          <p class="v4-section-label">02 / {{ t('home.v4.compat.label') }}</p>
          <h2 class="v4-section-title mt-4">{{ t('home.v4.compat.title') }}</h2>
          <p class="mt-5 text-sm leading-7 text-black/55 dark:text-white/55">{{ t('home.v4.compat.description') }}</p>
        </div>

        <div class="v4-protocol-grid mt-10">
          <div class="v4-protocol-card"><span class="v4-protocol-icon"><Icon name="chat" size="sm" /></span><strong>{{ t('home.v4.compat.openaiTitle') }}</strong><p>{{ t('home.v4.compat.openaiDesc') }}</p><code>/v1/chat/completions</code></div>
          <div class="v4-protocol-card"><span class="v4-protocol-icon"><Icon name="bolt" size="sm" /></span><strong>{{ t('home.v4.compat.responsesTitle') }}</strong><p>{{ t('home.v4.compat.responsesDesc') }}</p><code>/v1/responses</code></div>
          <div class="v4-protocol-card"><span class="v4-protocol-icon"><Icon name="terminal" size="sm" /></span><strong>{{ t('home.v4.compat.anthropicTitle') }}</strong><p>{{ t('home.v4.compat.anthropicDesc') }}</p><code>/v1/messages</code></div>
          <div class="v4-protocol-card"><span class="v4-protocol-icon"><Icon name="sparkles" size="sm" /></span><strong>{{ t('home.v4.compat.geminiTitle') }}</strong><p>{{ t('home.v4.compat.geminiDesc') }}</p><code>generateContent</code></div>
        </div>
      </section>

      <section class="v4-band border-y border-black/8 dark:border-white/8">
        <div class="mx-auto grid max-w-[1280px] gap-10 px-5 py-16 sm:px-8 lg:grid-cols-[.72fr_1.28fr] lg:gap-20 lg:px-10 lg:py-20">
          <div><p class="v4-section-label">03 / {{ t('home.v4.path.label') }}</p><h2 class="v4-section-title mt-4">{{ t('home.v4.path.title') }}</h2><p class="mt-5 max-w-sm text-sm leading-7 text-black/55 dark:text-white/55">{{ t('home.v4.path.description') }}</p></div>
          <div class="v4-path-list">
            <RouterLink to="/model-plaza" class="v4-path-row"><span>01</span><div><strong>{{ t('home.v4.path.modelsTitle') }}</strong><p>{{ t('home.v4.path.modelsDesc') }}</p></div><Icon name="arrowUp" size="sm" /></RouterLink>
            <RouterLink :to="memberPath('/creation')" class="v4-path-row"><span>02</span><div><strong>{{ t('home.v4.path.creationTitle') }}</strong><p>{{ t('home.v4.path.creationDesc') }}</p></div><Icon name="arrowUp" size="sm" /></RouterLink>
            <RouterLink :to="memberPath('/purchase')" class="v4-path-row"><span>03</span><div><strong>{{ t('home.v4.path.pricingTitle') }}</strong><p>{{ t('home.v4.path.pricingDesc') }}</p></div><Icon name="arrowUp" size="sm" /></RouterLink>
          </div>
        </div>
      </section>

      <section class="v4-trust-section">
        <div class="mx-auto max-w-[1280px] px-5 py-16 sm:px-8 lg:px-10 lg:py-20">
          <div class="grid gap-8 lg:grid-cols-[.8fr_1.2fr] lg:items-end">
            <div><p class="v4-section-label v4-section-label-light">04 / {{ t('home.v4.trust.label') }}</p><h2 class="v4-section-title mt-4 text-white">{{ t('home.v4.trust.title') }}</h2><p class="mt-5 max-w-md text-sm leading-7 text-white/58">{{ t('home.v4.trust.description') }}</p></div>
            <div class="v4-trust-grid">
              <div><Icon name="creditCard" size="md" /><strong>{{ t('home.v4.trust.item1Title') }}</strong><p>{{ t('home.v4.trust.item1Desc') }}</p></div>
              <div><Icon name="chartBar" size="md" /><strong>{{ t('home.v4.trust.item2Title') }}</strong><p>{{ t('home.v4.trust.item2Desc') }}</p></div>
              <div><Icon name="shield" size="md" /><strong>{{ t('home.v4.trust.item3Title') }}</strong><p>{{ t('home.v4.trust.item3Desc') }}</p></div>
            </div>
          </div>

          <div class="v4-final-cta mt-14">
            <div><strong>{{ t('home.v4.cta.title') }}</strong><p>{{ t('home.v4.cta.description') }}</p></div>
            <RouterLink to="/model-plaza" class="v4-light-button">{{ t('home.v4.cta.action') }}<Icon name="arrowRight" size="sm" /></RouterLink>
          </div>
        </div>
      </section>
    </main>

    <footer class="border-t border-black/8 px-5 py-8 text-xs text-black/45 dark:border-white/8 dark:text-white/45 sm:px-8 lg:px-10"><div class="mx-auto flex max-w-[1280px] flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><span>© {{ currentYear }} {{ siteName }}</span><span>{{ t('home.v4.footer') }}</span><RouterLink to="/model-plaza" class="v4-footer-link">{{ t('home.v4.nav.models') }} ↗</RouterLink></div></footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import HomeView from '@/views/HomeView.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

type Surface = 'models' | 'creation' | 'usage'
const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const activeSurface = ref<Surface>('models')
const isDark = ref(document.documentElement.classList.contains('dark'))
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'anytoken')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const hasHomeContent = computed(() => (appStore.cachedPublicSettings?.home_content || '').trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const userInitial = computed(() => authStore.user?.email?.charAt(0).toUpperCase() || '')
const currentYear = computed(() => new Date().getFullYear())
const activeSurfaceTitle = computed(() => t(`home.v4.surface.${activeSurface.value}Title`))

function memberPath(path: string) { return isAuthenticated.value ? path : '/login' }
function toggleTheme() { isDark.value = !isDark.value; document.documentElement.classList.toggle('dark', isDark.value); localStorage.setItem('theme', isDark.value ? 'dark' : 'light') }
function initTheme() { const savedTheme = localStorage.getItem('theme'); if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) { isDark.value = true; document.documentElement.classList.add('dark') } }
onMounted(() => { initTheme(); authStore.checkAuth(); if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings() })
</script>

<style scoped>
.v4-page { position: relative; isolation: isolate; overflow: hidden; }
.v4-page::before { position: absolute; inset: 0 0 auto; height: 640px; background-image: linear-gradient(rgba(92, 158, 60, .035) 1px, transparent 1px), linear-gradient(90deg, rgba(92, 158, 60, .035) 1px, transparent 1px); background-size: 44px 44px; content: ''; mask-image: linear-gradient(to bottom, black, transparent); pointer-events: none; }
.v4-nav { border: 1px solid rgba(23, 23, 23, .09); border-radius: 14px; background: rgba(255, 255, 255, .72); box-shadow: 0 10px 35px rgba(23, 23, 23, .06); backdrop-filter: blur(22px) saturate(145%); }
html.dark .v4-nav { border-color: rgba(255, 255, 255, .11); background: rgba(25, 26, 28, .72); box-shadow: 0 10px 35px rgba(0, 0, 0, .14); }
.v4-logo { display: inline-flex; height: 29px; width: 29px; align-items: center; justify-content: center; overflow: hidden; border: 1px solid rgba(23, 23, 23, .1); border-radius: 8px; background: white; padding: 4px; }
html.dark .v4-logo { border-color: rgba(255, 255, 255, .13); background: #111214; }
.v4-nav-link, .v4-footer-link { transition: color 180ms ease-out; }
.v4-nav-link:hover, .v4-footer-link:hover { color: #4e8d37; }
.v4-icon-button { display: inline-flex; height: 32px; width: 32px; align-items: center; justify-content: center; border-radius: 8px; color: rgba(23, 23, 23, .55); transition: color 160ms ease-out, background 160ms ease-out, transform 160ms ease-out; }
.v4-icon-button:hover { background: rgba(23, 23, 23, .06); color: #1d1d1f; }
.v4-icon-button:active, .v4-account-link:active, .v4-primary-button:active, .v4-secondary-button:active, .v4-light-button:active { transform: scale(.97); }
html.dark .v4-icon-button { color: rgba(255, 255, 255, .62); }
html.dark .v4-icon-button:hover { background: rgba(255, 255, 255, .08); color: white; }
.v4-account-link { display: inline-flex; align-items: center; gap: 6px; border-radius: 999px; background: #1d1d1f; padding: 8px 11px 8px 13px; color: white; font-size: 11px; font-weight: 700; transition: transform 160ms ease-out, background 180ms ease-out; }
.v4-account-link:hover { background: #4e8d37; }
html.dark .v4-account-link { background: #b5ef82; color: #14200f; }
html.dark .v4-account-link:hover { background: #c7f69d; }
.v4-entry { animation: v4-enter 620ms cubic-bezier(.23, 1, .32, 1) both; }
@keyframes v4-enter { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }
.v4-eyebrow { display: inline-flex; align-items: center; gap: 8px; color: rgba(23, 23, 23, .52); font-size: 11px; font-weight: 600; letter-spacing: 0; }
.v4-eyebrow span { height: 7px; width: 7px; border-radius: 999px; background: #5c9e3c; box-shadow: 0 0 0 5px rgba(92, 158, 60, .12); }
html.dark .v4-eyebrow { color: rgba(255, 255, 255, .54); }
.v4-title { max-width: 520px; font-size: 2.25rem; font-weight: 560; letter-spacing: 0; line-height: 1.16; }
.v4-title em { display: block; margin-top: 4px; color: #5c9e3c; font-style: normal; }
html.dark .v4-title em { color: #b5ef82; }
.v4-primary-button, .v4-secondary-button, .v4-light-button { display: inline-flex; align-items: center; gap: 9px; border-radius: 999px; font-size: 13px; font-weight: 700; transition: transform 160ms ease-out, background 180ms ease-out, box-shadow 180ms ease-out; }
.v4-primary-button { background: #1d1d1f; padding: 13px 16px 13px 19px; color: white; box-shadow: 0 12px 24px rgba(23, 23, 23, .14); }
.v4-primary-button:hover { background: #4e8d37; box-shadow: 0 16px 28px rgba(78, 141, 55, .2); }
html.dark .v4-primary-button { background: #b5ef82; color: #14200f; box-shadow: 0 12px 26px rgba(181, 239, 130, .1); }
.v4-secondary-button { border: 1px solid rgba(23, 23, 23, .12); padding: 12px 17px; color: rgba(23, 23, 23, .72); }
.v4-secondary-button:hover { background: rgba(23, 23, 23, .06); }
html.dark .v4-secondary-button { border-color: rgba(255, 255, 255, .15); color: rgba(255, 255, 255, .75); }
html.dark .v4-secondary-button:hover { background: rgba(255, 255, 255, .08); }
.v4-product-window { position: relative; overflow: hidden; border: 1px solid rgba(23, 23, 23, .12); border-radius: 18px; background: rgba(255, 255, 255, .82); box-shadow: 0 28px 65px rgba(23, 23, 23, .12), inset 0 1px rgba(255, 255, 255, .9); backdrop-filter: blur(24px) saturate(120%); }
html.dark .v4-product-window { border-color: rgba(255, 255, 255, .12); background: rgba(25, 26, 28, .82); box-shadow: 0 30px 70px rgba(0, 0, 0, .24), inset 0 1px rgba(255, 255, 255, .09); }
.v4-window-toolbar { display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; border-bottom: 1px solid rgba(23, 23, 23, .08); padding: 13px 17px; color: rgba(23, 23, 23, .45); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 9px; letter-spacing: 0; text-transform: uppercase; }
html.dark .v4-window-toolbar { border-color: rgba(255, 255, 255, .09); color: rgba(255, 255, 255, .44); }
.v4-window-toolbar > span:nth-child(2) { justify-self: center; }
.v4-live-indicator { display: inline-flex; align-items: center; justify-self: end; gap: 6px; }
.v4-live-indicator b, .v4-surface-chip b { display: block; height: 6px; width: 6px; border-radius: 999px; background: #79bf4e; box-shadow: 0 0 0 4px rgba(121, 191, 78, .12); }
.v4-dot { display: block; height: 7px; width: 7px; border-radius: 999px; }
.v4-dot-red { background: #ff766b; }
.v4-dot-yellow { background: #f2c75c; }
.v4-dot-green { background: #6fcf83; }
.v4-window-sidebar { position: relative; min-height: 330px; }
.v4-sidebar-label, .v4-overline { color: rgba(23, 23, 23, .4); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 9px; letter-spacing: 0; text-transform: uppercase; }
html.dark .v4-sidebar-label, html.dark .v4-overline { color: rgba(255, 255, 255, .4); }
.v4-sidebar-item { display: flex; width: 100%; align-items: center; gap: 9px; border-radius: 8px; padding: 9px 10px; color: rgba(23, 23, 23, .5); font-size: 12px; text-align: left; transition: color 160ms ease-out, background 160ms ease-out, transform 160ms ease-out; }
.v4-sidebar-item:hover { background: rgba(23, 23, 23, .05); color: #1d1d1f; }
.v4-sidebar-item:active { transform: scale(.98); }
.v4-sidebar-item-active { background: rgba(132, 195, 92, .14); color: #4e8d37; font-weight: 600; }
html.dark .v4-sidebar-item { color: rgba(255, 255, 255, .52); }
html.dark .v4-sidebar-item:hover { color: white; background: rgba(255, 255, 255, .06); }
html.dark .v4-sidebar-item-active { color: #b5ef82; background: rgba(181, 239, 130, .11); }
.v4-sidebar-bottom { position: absolute; right: 20px; bottom: 20px; left: 20px; display: flex; align-items: center; gap: 8px; color: rgba(23, 23, 23, .48); font-size: 10px; }
.v4-sidebar-bottom svg { margin-left: auto; }
html.dark .v4-sidebar-bottom { color: rgba(255, 255, 255, .48); }
.v4-avatar { display: inline-flex; height: 25px; width: 25px; align-items: center; justify-content: center; border-radius: 8px; background: #b5ef82; color: #18220f; font-size: 9px; font-weight: 800; }
.v4-surface-chip { display: inline-flex; align-items: center; gap: 7px; border: 1px solid rgba(121, 191, 78, .2); border-radius: 999px; background: rgba(121, 191, 78, .08); padding: 6px 9px; color: #5c9e3c; font-size: 10px; font-weight: 600; }
.v4-mobile-tabs { display: flex; gap: 5px; overflow-x: auto; border-bottom: 1px solid rgba(23, 23, 23, .08); padding-bottom: 7px; }
.v4-mobile-tabs button { flex: 0 0 auto; border-radius: 8px; padding: 7px 9px; color: rgba(23, 23, 23, .5); font-size: 11px; }
.v4-mobile-tab-active { background: rgba(132, 195, 92, .14); color: #4e8d37 !important; font-weight: 600; }
html.dark .v4-mobile-tabs { border-color: rgba(255, 255, 255, .1); }
html.dark .v4-mobile-tabs button { color: rgba(255, 255, 255, .5); }
html.dark .v4-mobile-tab-active { background: rgba(181, 239, 130, .11); color: #b5ef82 !important; }
.v4-surface-content { animation: v4-surface-in 220ms cubic-bezier(.23, 1, .32, 1) both; }
@keyframes v4-surface-in { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: translateY(0); } }
.v4-model-card { position: relative; display: flex; min-height: 148px; flex-direction: column; border: 1px solid rgba(23, 23, 23, .1); border-radius: 8px; padding: 14px; transition: transform 180ms ease-out, border-color 180ms ease-out, box-shadow 180ms ease-out; }
.v4-model-card:hover { border-color: rgba(92, 158, 60, .55); box-shadow: 0 12px 24px rgba(23, 23, 23, .07); transform: translateY(-2px); }
.v4-model-card:active { transform: scale(.98); }
.v4-model-card-active { border-color: rgba(92, 158, 60, .42); background: rgba(132, 195, 92, .08); }
html.dark .v4-model-card { border-color: rgba(255, 255, 255, .11); }
html.dark .v4-model-card-active { border-color: rgba(181, 239, 130, .38); background: rgba(181, 239, 130, .07); }
html.dark .v4-model-card:hover { border-color: rgba(181, 239, 130, .55); box-shadow: 0 12px 24px rgba(0, 0, 0, .16); }
.v4-model-symbol { display: inline-flex; height: 29px; width: 29px; align-items: center; justify-content: center; border-radius: 8px; color: white; font-size: 11px; font-weight: 800; }
.v4-symbol-orange { background: #d66c2f; }
.v4-symbol-green { background: #34885a; }
.v4-symbol-blue { background: #537cc5; }
.v4-model-card strong { margin-top: 15px; font-size: 14px; font-weight: 600; letter-spacing: 0; }
.v4-model-card small { margin-top: 4px; color: rgba(23, 23, 23, .46); font-size: 10px; }
.v4-model-price { margin-top: auto; color: #5c9e3c; font-size: 10px; font-weight: 600; }
html.dark .v4-model-card small { color: rgba(255, 255, 255, .48); }
.v4-surface-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; border-top: 1px solid rgba(23, 23, 23, .08); margin-top: 20px; padding-top: 16px; color: rgba(23, 23, 23, .44); font-size: 10px; }
.v4-surface-footer a { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 5px; color: #5c9e3c; font-weight: 600; }
.v4-surface-footer a:hover { color: #3f7c2a; }
html.dark .v4-surface-footer { border-color: rgba(255, 255, 255, .1); color: rgba(255, 255, 255, .45); }
.v4-creation-preview { position: relative; display: flex; min-height: 148px; align-items: flex-end; overflow: hidden; border-radius: 8px; background: #202225; padding: 18px; color: white; }
.v4-creation-grid { position: absolute; top: 16px; right: 16px; display: grid; width: 55%; grid-template-columns: 1fr 1fr; gap: 6px; }
.v4-creation-grid i { display: block; height: 48px; border-radius: 6px; background: #5c9e3c; opacity: .9; }
.v4-creation-grid i:nth-child(2) { background: #d9b46f; }
.v4-creation-grid i:nth-child(3) { grid-column: span 2; height: 30px; background: #7186a6; }
.v4-creation-play { position: absolute; top: 18px; left: 18px; display: inline-flex; height: 30px; width: 30px; align-items: center; justify-content: center; border: 1px solid rgba(255, 255, 255, .2); border-radius: 999px; background: rgba(255, 255, 255, .1); color: #c8f69b; }
.v4-creation-preview p { position: relative; color: rgba(255, 255, 255, .55); font-size: 10px; }
.v4-creation-preview strong { position: relative; display: block; margin-top: 5px; font-size: 18px; letter-spacing: 0; }
.v4-usage-preview { display: flex; min-height: 148px; flex-direction: column; justify-content: space-between; border: 1px solid rgba(23, 23, 23, .1); border-radius: 8px; background: rgba(132, 195, 92, .08); padding: 17px; }
.v4-usage-preview span { color: rgba(23, 23, 23, .45); font-size: 10px; }
.v4-usage-preview strong { display: block; margin-top: 8px; font-size: 26px; font-weight: 600; letter-spacing: 0; }
.v4-usage-bars { display: flex; height: 42px; align-items: end; gap: 7px; }
.v4-usage-bars i { display: block; flex: 1; border-radius: 4px 4px 2px 2px; background: #84c35c; }
html.dark .v4-usage-preview { border-color: rgba(255, 255, 255, .1); background: rgba(181, 239, 130, .08); }
html.dark .v4-usage-preview span { color: rgba(255, 255, 255, .48); }
.v4-proof-strip { display: grid; border-top: 1px solid rgba(23, 23, 23, .1); border-bottom: 1px solid rgba(23, 23, 23, .1); }
.v4-proof-strip > div { display: flex; align-items: center; gap: 12px; padding: 17px 2px; }
.v4-proof-strip > div + div { border-top: 1px solid rgba(23, 23, 23, .1); }
.v4-proof-strip svg { color: #5c9e3c; }
.v4-proof-strip span { display: flex; min-width: 0; flex-direction: column; gap: 2px; }
.v4-proof-strip strong { font-size: 12px; font-weight: 650; }
.v4-proof-strip small { color: rgba(23, 23, 23, .48); font-size: 10px; line-height: 1.5; }
html.dark .v4-proof-strip, html.dark .v4-proof-strip > div + div { border-color: rgba(255, 255, 255, .1); }
html.dark .v4-proof-strip small { color: rgba(255, 255, 255, .48); }
.v4-band { background: rgba(255, 255, 255, .36); }
html.dark .v4-band { background: rgba(255, 255, 255, .025); }
.v4-section-label { color: #5c9e3c; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 10px; font-weight: 700; letter-spacing: 0; text-transform: uppercase; }
.v4-section-title { max-width: 550px; font-size: 1.75rem; font-weight: 600; letter-spacing: 0; line-height: 1.2; }
.v4-flow { display: grid; align-items: stretch; gap: 12px; }
.v4-flow-column { border: 1px solid rgba(23, 23, 23, .1); border-radius: 8px; background: rgba(255, 255, 255, .56); padding: 14px; }
.v4-flow-column > p, .v4-gateway > p { color: rgba(23, 23, 23, .42); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 9px; text-transform: uppercase; }
.v4-flow-list { display: grid; gap: 7px; margin-top: 12px; }
.v4-flow-list span { display: flex; min-height: 34px; align-items: center; gap: 8px; border-radius: 7px; background: rgba(23, 23, 23, .045); padding: 8px 9px; color: rgba(23, 23, 23, .7); font-size: 11px; }
.v4-flow-list svg { color: #5c9e3c; }
.v4-flow-arrow { display: flex; min-height: 36px; align-items: center; justify-content: center; gap: 7px; color: #5c9e3c; }
.v4-flow-arrow span { display: none; }
.v4-flow-arrow em { color: rgba(23, 23, 23, .38); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 8px; font-style: normal; }
.v4-gateway { position: relative; border: 1px solid rgba(92, 158, 60, .36); border-radius: 8px; background: rgba(132, 195, 92, .1); padding: 16px; }
.v4-gateway-mark { display: inline-flex; height: 34px; width: 34px; align-items: center; justify-content: center; border-radius: 8px; background: #5c9e3c; color: white; }
.v4-gateway > p { margin-top: 18px; color: #5c9e3c; }
.v4-gateway > strong { display: block; margin-top: 4px; font-size: 15px; }
.v4-gateway > small { display: block; margin-top: 6px; color: rgba(23, 23, 23, .5); font-size: 10px; line-height: 1.6; }
.v4-gateway ul { display: grid; gap: 6px; margin-top: 14px; }
.v4-gateway li { display: flex; align-items: center; gap: 6px; color: rgba(23, 23, 23, .66); font-size: 10px; }
.v4-gateway li svg { color: #5c9e3c; }
.v4-provider-dot { display: block; height: 8px; width: 8px; border-radius: 999px; }
.v4-provider-orange { background: #d66c2f; }
.v4-provider-green { background: #34885a; }
.v4-provider-blue { background: #537cc5; }
.v4-provider-gray { background: #8b8d90; }
html.dark .v4-flow-column { border-color: rgba(255, 255, 255, .1); background: rgba(255, 255, 255, .035); }
html.dark .v4-flow-column > p, html.dark .v4-gateway > p, html.dark .v4-flow-arrow em { color: rgba(255, 255, 255, .42); }
html.dark .v4-flow-list span { background: rgba(255, 255, 255, .055); color: rgba(255, 255, 255, .7); }
html.dark .v4-gateway { border-color: rgba(181, 239, 130, .3); background: rgba(181, 239, 130, .075); }
html.dark .v4-gateway > p { color: #b5ef82; }
html.dark .v4-gateway > small, html.dark .v4-gateway li { color: rgba(255, 255, 255, .54); }
.v4-protocol-grid { display: grid; gap: 10px; }
.v4-protocol-card { display: flex; min-height: 190px; flex-direction: column; border: 1px solid rgba(23, 23, 23, .1); border-radius: 8px; background: rgba(255, 255, 255, .52); padding: 16px; transition: border-color 180ms ease-out, transform 180ms ease-out, box-shadow 180ms ease-out; }
.v4-protocol-card:hover { border-color: rgba(92, 158, 60, .45); box-shadow: 0 12px 30px rgba(23, 23, 23, .06); transform: translateY(-2px); }
.v4-protocol-icon { display: inline-flex; height: 32px; width: 32px; align-items: center; justify-content: center; border-radius: 8px; background: rgba(92, 158, 60, .1); color: #5c9e3c; }
.v4-protocol-card strong { margin-top: 20px; font-size: 14px; font-weight: 650; }
.v4-protocol-card p { margin-top: 7px; color: rgba(23, 23, 23, .5); font-size: 11px; line-height: 1.65; }
.v4-protocol-card code { display: block; overflow: hidden; margin-top: auto; padding-top: 16px; color: #5c9e3c; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
html.dark .v4-protocol-card { border-color: rgba(255, 255, 255, .1); background: rgba(255, 255, 255, .035); }
html.dark .v4-protocol-card:hover { border-color: rgba(181, 239, 130, .42); box-shadow: 0 12px 30px rgba(0, 0, 0, .16); }
html.dark .v4-protocol-card p { color: rgba(255, 255, 255, .5); }
html.dark .v4-protocol-card code { color: #b5ef82; }
.v4-path-list { border-top: 1px solid rgba(23, 23, 23, .1); }
.v4-path-row { display: grid; grid-template-columns: 36px 1fr auto; align-items: start; gap: 14px; border-bottom: 1px solid rgba(23, 23, 23, .1); padding: 21px 0; color: inherit; transition: padding 180ms ease-out, color 180ms ease-out; }
.v4-path-row:hover { padding-left: 8px; color: #5c9e3c; }
.v4-path-row > span { color: #5c9e3c; font-family: ui-monospace, monospace; font-size: 10px; }
.v4-path-row strong { font-size: 16px; font-weight: 600; letter-spacing: 0; }
.v4-path-row p { margin-top: 5px; color: rgba(23, 23, 23, .5); font-size: 12px; line-height: 1.6; }
.v4-path-row > svg { opacity: 0; transition: opacity 180ms ease-out, transform 180ms ease-out; }
.v4-path-row:hover > svg { opacity: 1; transform: translateY(-2px); }
html.dark .v4-path-row { border-color: rgba(255, 255, 255, .1); }
html.dark .v4-path-row p { color: rgba(255, 255, 255, .5); }
.v4-trust-section { background: #1d1d1f; color: white; }
.v4-section-label-light { color: #9ed273; }
.v4-trust-grid { display: grid; border-top: 1px solid rgba(255, 255, 255, .12); }
.v4-trust-grid > div { display: flex; min-height: 150px; flex-direction: column; border-bottom: 1px solid rgba(255, 255, 255, .12); padding: 20px 0; }
.v4-trust-grid svg { color: #b5ef82; }
.v4-trust-grid strong { margin-top: 28px; font-size: 13px; }
.v4-trust-grid p { margin-top: 7px; color: rgba(255, 255, 255, .52); font-size: 11px; line-height: 1.65; }
.v4-final-cta { display: flex; flex-direction: column; gap: 22px; border-top: 1px solid rgba(255, 255, 255, .13); padding-top: 30px; }
.v4-final-cta strong { font-size: 19px; font-weight: 600; }
.v4-final-cta p { margin-top: 6px; color: rgba(255, 255, 255, .5); font-size: 12px; line-height: 1.6; }
.v4-light-button { align-self: flex-start; background: #b5ef82; padding: 12px 16px 12px 18px; color: #14200f; }
.v4-light-button:hover { background: #c7f69d; }
@media (min-width: 640px) {
  .v4-title { font-size: 2.625rem; }
  .v4-proof-strip { grid-template-columns: repeat(3, 1fr); }
  .v4-proof-strip > div { padding: 17px 18px; }
  .v4-proof-strip > div + div { border-top: 0; border-left: 1px solid rgba(23, 23, 23, .1); }
  html.dark .v4-proof-strip > div + div { border-left-color: rgba(255, 255, 255, .1); }
  .v4-protocol-grid { grid-template-columns: repeat(2, 1fr); }
  .v4-trust-grid { grid-template-columns: repeat(3, 1fr); }
  .v4-trust-grid > div { border-right: 1px solid rgba(255, 255, 255, .12); padding: 20px; }
  .v4-trust-grid > div:first-child { padding-left: 0; }
  .v4-trust-grid > div:last-child { border-right: 0; padding-right: 0; }
  .v4-final-cta { flex-direction: row; align-items: center; justify-content: space-between; }
  .v4-light-button { align-self: center; }
}
@media (min-width: 1024px) {
  .v4-flow { grid-template-columns: minmax(120px, .9fr) 42px minmax(175px, 1.12fr) 42px minmax(120px, .9fr); }
  .v4-flow-arrow { align-self: center; }
  .v4-flow-arrow span { display: block; height: 1px; flex: 1; background: rgba(92, 158, 60, .3); }
  .v4-flow-arrow em { display: none; }
  .v4-protocol-grid { grid-template-columns: repeat(4, 1fr); }
}
@media (prefers-reduced-motion: reduce) {
  .v4-entry, .v4-surface-content { animation: none; }
  .v4-nav-link, .v4-icon-button, .v4-account-link, .v4-primary-button, .v4-secondary-button, .v4-light-button, .v4-sidebar-item, .v4-model-card, .v4-protocol-card, .v4-path-row, .v4-path-row > svg { transition: color 160ms ease, background 160ms ease, border-color 160ms ease !important; transform: none !important; }
}
@media (prefers-reduced-transparency: reduce) {
  .v4-nav, .v4-product-window { background: #fff; backdrop-filter: none; }
  html.dark .v4-nav, html.dark .v4-product-window { background: #191a1c; }
}
</style>
