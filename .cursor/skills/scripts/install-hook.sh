#!/usr/bin/env bash
# 一键安装 pre-commit 钩子：把 .cursor/skills/scripts/check-sensitive.sh 软链到 .git/hooks/pre-commit
#
# 用法：
#   bash .cursor/skills/scripts/install-hook.sh
#
# 行为：
#   - 如果已存在非软链的 pre-commit，先备份为 pre-commit.bak.<timestamp>
#   - 如果已存在软链，会被覆盖（-f 选项）
#   - 不会修改 git config

set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || {
  echo "错误：当前目录不是一个 Git 仓库" >&2
  exit 1
})

HOOK_DIR="$ROOT/.git/hooks"
HOOK="$HOOK_DIR/pre-commit"
SCRIPT_REL=".cursor/skills/scripts/check-sensitive.sh"
SCRIPT_ABS="$ROOT/$SCRIPT_REL"
# 软链内容使用相对路径，这样仓库换位置也能跟着走
LINK_TARGET="../../$SCRIPT_REL"

if [[ ! -f "$SCRIPT_ABS" ]]; then
  echo "错误：未找到 $SCRIPT_REL" >&2
  exit 1
fi

mkdir -p "$HOOK_DIR"
chmod +x "$SCRIPT_ABS"

if [[ -e "$HOOK" && ! -L "$HOOK" ]]; then
  backup="$HOOK.bak.$(date +%s)"
  mv "$HOOK" "$backup"
  echo "已备份原有 pre-commit -> $backup"
fi

ln -sfn "$LINK_TARGET" "$HOOK"

echo "安装完成。"
echo "  软链：$HOOK -> $LINK_TARGET"
echo
echo "下一次 git commit 会自动扫描 .cursor/skills/ 下的 markdown。"
echo "紧急跳过：SKIP_SENSITIVE_SCAN=1 git commit ..."
echo "手动全量扫描：bash $SCRIPT_REL"
