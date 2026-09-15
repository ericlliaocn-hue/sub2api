#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
check="${repo_root}/deploy/check-site.sh"
robots="${repo_root}/frontend/public/robots.txt"

if grep -Fq 'oioio.chat' "$robots"; then
  "$check" oioio >/dev/null
  if "$check" anytoken >/dev/null 2>&1; then
    echo "oioio 检出不应通过 anytoken 检查" >&2
    exit 1
  fi
  echo "check-site-test: oioio checkout rejects anytoken and accepts oioio"
else
  "$check" anytoken >/dev/null
  if "$check" oioio >/dev/null 2>&1; then
    echo "anytoken 检出不应通过 oioio 检查" >&2
    exit 1
  fi
  echo "check-site-test: anytoken checkout rejects oioio and accepts anytoken"
fi
