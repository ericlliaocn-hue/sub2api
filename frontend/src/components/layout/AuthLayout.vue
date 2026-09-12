<template>
  <div class="auth-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
    <!-- Background -->
    <div
      class="absolute inset-0 bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950"
    ></div>

    <!-- Decorative Elements -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <!-- Gradient Orbs -->
      <div
        class="auth-orb auth-orb-right absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/20 blur-3xl"
      ></div>
      <div
        class="auth-orb auth-orb-left absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-[#ff9a6b]/20 blur-3xl"
      ></div>
      <div
        class="auth-orb auth-orb-center absolute left-1/2 top-1/2 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-300/10 blur-3xl"
      ></div>

    </div>

    <!-- Content Container -->
    <div class="relative z-10 w-full max-w-md">
      <!-- Logo/Brand -->
      <div class="auth-brand mb-8 text-center">
        <!-- Custom Logo or Default Logo -->
        <template v-if="settingsLoaded">
          <div
            class="auth-logo mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl shadow-lg shadow-primary-500/30"
          >
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1 class="text-gradient mb-2 text-3xl font-bold">
            {{ siteName }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <!-- Card Container -->
      <div class="auth-card card-glass rounded-2xl p-8 shadow-glass">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-page {
  --auth-ease-out: cubic-bezier(0.23, 1, 0.32, 1);
  --auth-ease-in-out: cubic-bezier(0.77, 0, 0.175, 1);
}

.auth-brand {
  animation: auth-enter 220ms var(--auth-ease-out) 40ms both;
}

.auth-card {
  animation: auth-enter 240ms var(--auth-ease-out) 100ms both;
}

.auth-logo {
  animation: auth-logo-breathe 4s var(--auth-ease-in-out) infinite;
}

.auth-orb {
  will-change: transform, opacity;
}

.auth-orb-right {
  animation: auth-orb-drift-right 14s var(--auth-ease-in-out) infinite alternate;
}

.auth-orb-left {
  animation: auth-orb-drift-left 16s var(--auth-ease-in-out) infinite alternate-reverse;
}

.auth-orb-center {
  animation: auth-orb-pulse 8s var(--auth-ease-in-out) infinite;
}

@keyframes auth-enter {
  from {
    opacity: 0;
    transform: translateY(8px) scale(0.99);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes auth-logo-breathe {
  0%,
  100% {
    transform: translateY(0) scale(1);
  }
  50% {
    transform: translateY(-2px) scale(1.015);
  }
}

@keyframes auth-orb-drift-right {
  from {
    opacity: 0.72;
    transform: translate3d(0, 0, 0);
  }
  to {
    opacity: 1;
    transform: translate3d(-14px, 10px, 0);
  }
}

@keyframes auth-orb-drift-left {
  from {
    opacity: 0.68;
    transform: translate3d(0, 0, 0);
  }
  to {
    opacity: 0.92;
    transform: translate3d(14px, -10px, 0);
  }
}

@keyframes auth-orb-pulse {
  0%,
  100% {
    opacity: 0.55;
  }
  50% {
    opacity: 0.9;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-brand,
  .auth-card {
    animation: auth-fade-in 180ms var(--auth-ease-out) both;
  }

  .auth-logo,
  .auth-orb {
    animation: none;
  }
}

@keyframes auth-fade-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

.text-gradient {
  @apply bg-gradient-to-r from-primary-700 via-primary-600 to-[#ff9a6b] bg-clip-text text-transparent;
}
</style>
