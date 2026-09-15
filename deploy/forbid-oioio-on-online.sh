#!/usr/bin/env bash
# Block oioio skin from anytoken/main, and keep local oioio branches off remotes.
# Usage:
#   deploy/forbid-oioio-on-online.sh check-branch [branch]
#   deploy/forbid-oioio-on-online.sh check-tree [git-rev]
#   deploy/forbid-oioio-on-online.sh check-push <local-ref> <remote-ref>

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

fail() {
  echo "forbid-oioio: $*" >&2
  exit 1
}

is_online_branch() {
  local name="${1#refs/heads/}"
  case "$name" in
    main|master|ljx/feature/anytoken|ljx/feature/anytoken-*|ljx/feature/shared-main*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

is_local_only_oioio_branch() {
  local name="${1#refs/heads/}"
  case "$name" in
    ljx/feature/oioio-on-*|ljx/feature/oioio-ui-*|ljx/feature/oioio-public-homepage)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

tree_has_oioio_skin() {
  local rev="${1:-HEAD}"
  git grep -q 'api.oioio.chat' "$rev" -- frontend/src/views/HomeLandingV4View.vue 2>/dev/null \
    || git grep -q 'Sitemap: https://oioio.chat' "$rev" -- frontend/public/robots.txt 2>/dev/null
}

index_has_oioio_skin() {
  git diff --cached -U0 -- frontend/public/robots.txt frontend/src/views/HomeLandingV4View.vue \
    frontend/src/i18n/locales/zh/landing.ts frontend/src/router/seo.ts \
    | grep -E '^\+.*(api\.oioio\.chat|Sitemap: https://oioio\.chat|/oioio-logo\.svg)' >/dev/null
}

cmd="${1:-}"
case "$cmd" in
  check-branch)
    branch="${2:-$(git branch --show-current)}"
    if is_online_branch "$branch"; then
      if index_has_oioio_skin; then
        fail "线上分支 ${branch} 不能暂存 oioio 皮（api.oioio.chat / oioio.chat）。请改到本地 oioio 分支。"
      fi
      if tree_has_oioio_skin HEAD; then
        fail "线上分支 ${branch} 的 HEAD 已混入 oioio 皮，先撤掉再提交。"
      fi
    fi
    ;;
  check-tree)
    rev="${2:-HEAD}"
    if tree_has_oioio_skin "$rev"; then
      fail "${rev} 含 oioio 皮，不能进线上分支。"
    fi
    ;;
  check-push)
    local_ref="${2:-}"
    remote_ref="${3:-}"
    [ -n "$local_ref" ] && [ -n "$remote_ref" ] || fail "用法: check-push <local-ref> <remote-ref>"
    if is_local_only_oioio_branch "$local_ref"; then
      fail "${local_ref} 是本地 oioio 皮分支，不能推到远程/线上。发 oioio 在本机打另一包。"
    fi
    if is_online_branch "$remote_ref"; then
      if is_local_only_oioio_branch "$local_ref"; then
        fail "不能把 ${local_ref} 推到线上分支 ${remote_ref}。"
      fi
      local_sha="$(git rev-parse "$local_ref" 2>/dev/null || true)"
      if [ -n "$local_sha" ] && tree_has_oioio_skin "$local_sha"; then
        fail "推到 ${remote_ref} 的提交含 oioio 皮，已拒绝。"
      fi
    fi
    ;;
  *)
    fail "用法: deploy/forbid-oioio-on-online.sh check-branch|check-tree|check-push ..."
    ;;
esac
