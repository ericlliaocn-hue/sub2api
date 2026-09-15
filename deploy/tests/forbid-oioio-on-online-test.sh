#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
forbid="${repo_root}/deploy/forbid-oioio-on-online.sh"

"$forbid" check-branch ljx/feature/anytoken-0-2-4
if ! grep -Fq 'oioio.chat' "${repo_root}/frontend/public/robots.txt"; then
  "$forbid" check-tree HEAD
fi

if "$forbid" check-push ljx/feature/oioio-on-0-2-5 refs/heads/main >/dev/null 2>&1; then
  echo "应拒绝把 oioio 分支推到 main" >&2
  exit 1
fi
if "$forbid" check-push ljx/feature/oioio-on-0-2-5 refs/heads/ljx/feature/anytoken-0-2-4 >/dev/null 2>&1; then
  echo "应拒绝把 oioio 分支推到 anytoken" >&2
  exit 1
fi
if "$forbid" check-push ljx/feature/oioio-on-0-2-5 refs/heads/ljx/feature/oioio-on-0-2-5 >/dev/null 2>&1; then
  echo "应拒绝把本地 oioio 皮分支推到远程" >&2
  exit 1
fi
"$forbid" check-push ljx/feature/anytoken-0-2-4 refs/heads/ljx/feature/anytoken-0-2-4

echo "forbid-oioio-on-online-test: ok"
