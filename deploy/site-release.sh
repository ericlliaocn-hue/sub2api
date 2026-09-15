#!/usr/bin/env bash
# Print how to ship one site from this checkout. Never deploys.
# Usage: deploy/site-release.sh anytoken|oioio

set -euo pipefail

site="${1:-}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
check="${repo_root}/deploy/check-site.sh"

case "$site" in
  anytoken)
    "$check" anytoken
    echo
    echo "发 anytoken"
    echo "  1. 功能只在 anytoken 主线改。"
    echo "  2. 用当前检出打镜像。"
    echo "  3. 只上 anytoken.work / api.anytoken.work。"
    echo "  4. 不要上 oioio 机器，不要把 oioio 皮叠进主线。"
    echo "这个脚本不会部署。"
    ;;
  oioio)
    "$check" oioio
    echo
    echo "发 oioio"
    echo "  1. 在 oioio 分支上：主线功能已合入，首页/文案/颜色/接入域名仍是 oioio。"
    echo "  2. 用当前检出另打一包。"
    echo "  3. 只上 oioio.chat / api.oioio.chat。"
    echo "  4. 不要上 anytoken 机器，不要把 oioio 皮交回主线。"
    echo "这个脚本不会部署。"
    ;;
  *)
    echo "用法: deploy/site-release.sh anytoken|oioio" >&2
    exit 1
    ;;
esac
