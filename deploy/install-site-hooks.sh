#!/usr/bin/env bash
# Install local git hooks that keep oioio skin off online branches.
# Does not change git config.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
hook_dir="${repo_root}/.git/hooks"
src="${repo_root}/deploy/git-hooks"

if [ ! -d "$hook_dir" ]; then
  echo "install-site-hooks: ${hook_dir} 不存在，跳过" >&2
  exit 1
fi

install_hook() {
  local name="$1"
  cp "$src/$name" "$hook_dir/$name"
  chmod +x "$hook_dir/$name"
}

install_hook pre-commit
install_hook pre-push
echo "install-site-hooks: 已安装 pre-commit / pre-push。oioio 皮不能进 main/anytoken，也不能推本地 oioio 分支。"
