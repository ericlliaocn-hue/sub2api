#!/usr/bin/env bash
# Confirm the checkout skin matches one site. Does not build or deploy.
# Usage: deploy/check-site.sh anytoken|oioio

set -euo pipefail

site="${1:-}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
robots="${repo_root}/frontend/public/robots.txt"
landing="${repo_root}/frontend/src/i18n/locales/zh/landing.ts"
home="${repo_root}/frontend/src/views/HomeLandingV4View.vue"

fail() {
  echo "check-site: $*" >&2
  exit 1
}

has() {
  local file="$1" needle="$2"
  grep -Fq "$needle" "$file"
}

case "$site" in
  anytoken)
    has "$robots" "anytoken.work" || fail "anytoken 皮缺失：robots.txt 必须是 anytoken.work"
    has "$robots" "oioio.chat" && fail "anytoken 检出混入了 oioio robots"
    has "$landing" "AnyToken" || fail "anytoken 皮缺失：中文首页文案应含 AnyToken"
    has "$home" "api.oioio.chat" && fail "anytoken 检出混入了 oioio 接入地址"
    echo "check-site: anytoken 皮正确。只打 anytoken 包，只上 anytoken.work 那台机器。"
    ;;
  oioio)
    has "$robots" "oioio.chat" || fail "oioio 皮缺失：robots.txt 必须是 oioio.chat"
    has "$robots" "anytoken.work" && fail "oioio 检出混入了 anytoken robots"
    has "$home" "api.oioio.chat/v1" || fail "oioio 皮缺失：首页接入地址必须是 api.oioio.chat/v1"
    has "$landing" "AnyToken · AI中转站" && fail "oioio 检出还是 anytoken 首页文案"
    echo "check-site: oioio 皮正确。只打 oioio 包，只上 oioio.chat 那台机器。"
    ;;
  *)
    fail "用法: deploy/check-site.sh anytoken|oioio"
    ;;
esac
