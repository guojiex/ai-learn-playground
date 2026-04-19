#!/usr/bin/env bash
# 敏感词扫描脚本
#
# 两种运行方式：
#   1. 作为 Git pre-commit 钩子自动执行（扫 staged 内容，.cursor/skills/ 下的 md）
#   2. 手动执行：bash .cursor/skills/scripts/check-sensitive.sh
#      不带参数 —— 扫 .cursor/skills/ 下所有 md（工作区内容）
#      带文件路径 —— 只扫指定的文件
#
# 环境变量：
#   SKIP_SENSITIVE_SCAN=1   跳过全部检查（紧急逃生，不推荐）
#
# 配置文件：.cursor/skills/.sensitive-words.yaml
#
# 性能：每个文件最多只调用 2 次 grep（words 合并一次 / patterns 合并一次），
#       命中后再做 per-rule 归因（命中是小概率所以不再是瓶颈）。

set -euo pipefail

if [[ "${SKIP_SENSITIVE_SCAN:-}" == "1" ]]; then
  echo "[敏感词扫描] 已被 SKIP_SENSITIVE_SCAN=1 跳过" >&2
  exit 0
fi

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
CONFIG="$ROOT/.cursor/skills/.sensitive-words.yaml"
SCAN_DIR=".cursor/skills"

if [[ ! -f "$CONFIG" ]]; then
  echo "[敏感词扫描] 未找到配置文件：$CONFIG，跳过" >&2
  exit 0
fi

# ------------------------------------------------------------------
# 解析 yaml（只支持 words / patterns / allowlist 三个扁平 list）
# ------------------------------------------------------------------
WORDS=()
PATTERNS=()
ALLOWLIST=()

current=""
while IFS= read -r line || [[ -n "$line" ]]; do
  line="${line%$'\r'}"
  if [[ "$line" =~ ^[[:space:]]*# ]] || [[ -z "${line// /}" ]]; then
    continue
  fi
  if [[ "$line" =~ ^words:[[:space:]]*$ ]]; then
    current="words"; continue
  elif [[ "$line" =~ ^patterns:[[:space:]]*$ ]]; then
    current="patterns"; continue
  elif [[ "$line" =~ ^allowlist:[[:space:]]*$ ]]; then
    current="allowlist"; continue
  fi
  if [[ "$line" =~ ^[[:space:]]+-[[:space:]]+(.+)$ ]]; then
    item="${BASH_REMATCH[1]}"
    if [[ ! "$item" =~ ^[\"\'] ]]; then
      item="${item%%  #*}"
      item="${item%%	#*}"
    fi
    item="${item#"${item%%[![:space:]]*}"}"
    item="${item%"${item##*[![:space:]]}"}"
    if [[ "$item" =~ ^\"(.*)\"$ ]]; then
      item="${BASH_REMATCH[1]}"
    elif [[ "$item" =~ ^\'(.*)\'$ ]]; then
      item="${BASH_REMATCH[1]}"
    fi
    case "$current" in
      words) WORDS+=("$item") ;;
      patterns) PATTERNS+=("$item") ;;
      allowlist) ALLOWLIST+=("$item") ;;
    esac
  fi
done < "$CONFIG"

# ------------------------------------------------------------------
# 预构造合并正则：一次 grep 扫全部 words / 全部 patterns
# ------------------------------------------------------------------
WORDS_RE=""
if [[ ${#WORDS[@]} -gt 0 ]]; then
  joined=""
  for w in "${WORDS[@]}"; do
    [[ -z "$w" ]] && continue
    [[ -n "$joined" ]] && joined="${joined}|"
    joined="${joined}${w}"
  done
  # 单词边界：前后必须是非字母数字下划线
  [[ -n "$joined" ]] && WORDS_RE="(^|[^A-Za-z0-9_])(${joined})([^A-Za-z0-9_]|\$)"
fi

PATTERNS_RE=""
if [[ ${#PATTERNS[@]} -gt 0 ]]; then
  joined=""
  for p in "${PATTERNS[@]}"; do
    [[ -z "$p" ]] && continue
    [[ -n "$joined" ]] && joined="${joined}|"
    joined="${joined}(${p})"
  done
  [[ -n "$joined" ]] && PATTERNS_RE="$joined"
fi

# ------------------------------------------------------------------
# 收集待扫描文件列表
# ------------------------------------------------------------------
TARGET_FILES=()
MODE="workspace"   # staged | workspace | args

if [[ $# -gt 0 ]]; then
  MODE="args"
  for f in "$@"; do
    TARGET_FILES+=("$f")
  done
fi

# 自动识别 pre-commit 场景：没带参数 & 有 staged 的 md 文件
if [[ "$MODE" == "workspace" ]] && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  staged=$(git diff --cached --name-only --diff-filter=ACMR 2>/dev/null || true)
  if [[ -n "$staged" ]]; then
    while IFS= read -r f; do
      [[ -z "$f" ]] && continue
      if [[ "$f" =~ ^\.cursor/skills/.*\.md$ ]]; then
        TARGET_FILES+=("$f")
        MODE="staged"
      fi
    done <<< "$staged"
  fi
fi

# workspace 模式：扫 .cursor/skills/ 下所有 md
if [[ "$MODE" == "workspace" ]] && [[ ${#TARGET_FILES[@]} -eq 0 ]]; then
  while IFS= read -r f; do
    TARGET_FILES+=("$f")
  done < <(find "$ROOT/$SCAN_DIR" -type f -name "*.md" 2>/dev/null)
fi

if [[ ${#TARGET_FILES[@]} -eq 0 ]]; then
  exit 0
fi

# ------------------------------------------------------------------
# 扫描
# ------------------------------------------------------------------
hit_count=0
hit_output=""

# 把文件内容拿到一个临时文件（staged 模式要从 index 读；其它直接用磁盘路径）
TMPDIR_SCAN=$(mktemp -d)
trap 'rm -rf "$TMPDIR_SCAN"' EXIT

resolve_path_for_scan() {
  # 输出：一个可被 grep 读的路径
  local file="$1"
  if [[ "$MODE" == "staged" ]]; then
    local tmp="$TMPDIR_SCAN/$(echo "$file" | tr '/' '_')"
    if git show ":$file" > "$tmp" 2>/dev/null; then
      printf '%s' "$tmp"
    else
      printf ''
    fi
  else
    if [[ -f "$file" ]]; then
      printf '%s' "$file"
    elif [[ -f "$ROOT/$file" ]]; then
      printf '%s' "$ROOT/$file"
    else
      printf ''
    fi
  fi
}

line_in_allowlist() {
  local text="$1"
  local item
  for item in "${ALLOWLIST[@]:-}"; do
    [[ -z "$item" ]] && continue
    if [[ "$text" == *"$item"* ]]; then
      return 0
    fi
  done
  return 1
}

# 归因：对命中的单行，找出到底匹配了哪个词 / 哪条正则（命中是小概率路径）
attribute_rule() {
  local kind="$1" text="$2"
  if [[ "$kind" == "word" ]]; then
    local w
    for w in "${WORDS[@]}"; do
      [[ -z "$w" ]] && continue
      if printf '%s' "$text" | grep -Eqi "(^|[^A-Za-z0-9_])${w}([^A-Za-z0-9_]|\$)"; then
        printf '%s' "$w"
        return
      fi
    done
  else
    local p
    for p in "${PATTERNS[@]}"; do
      [[ -z "$p" ]] && continue
      if printf '%s' "$text" | grep -Eq "$p"; then
        printf '%s' "$p"
        return
      fi
    done
  fi
  printf '?'
}

add_hit() {
  local file="$1" line_no="$2" rule_type="$3" rule="$4" text="$5"
  local trimmed="${text#"${text%%[![:space:]]*}"}"
  trimmed="${trimmed%"${trimmed##*[![:space:]]}"}"
  if [[ ${#trimmed} -gt 120 ]]; then
    trimmed="${trimmed:0:117}..."
  fi
  hit_output+="  ${file}:${line_no}: 触发 ${rule_type}=\"${rule}\""$'\n'
  hit_output+="    > ${trimmed}"$'\n'
  hit_output+="    建议：改用代称（见 .cursor/skills/office-politics-combat/SKILL.md「角色代称」节），或按需在 .sensitive-words.yaml 的 allowlist 添加例外。"$'\n'
  hit_count=$((hit_count + 1))
}

# 对一个文件跑一条合并正则，把 grep 结果解析成 (line_no, text) 加入 hits
scan_one() {
  local file="$1" target="$2" kind="$3" regex="$4" grep_flags="$5"
  [[ -z "$regex" ]] && return 0
  [[ -z "$target" ]] && return 0
  local line
  # grep -nE[i] —— 一次扫完整个文件
  # shellcheck disable=SC2086
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    local line_no="${line%%:*}"
    local text="${line#*:}"
    if line_in_allowlist "$text"; then
      continue
    fi
    local rule
    rule=$(attribute_rule "$kind" "$text")
    add_hit "$file" "$line_no" "$kind" "$rule" "$text"
  done < <(grep -nE $grep_flags -- "$regex" "$target" 2>/dev/null || true)
}

for file in "${TARGET_FILES[@]}"; do
  target=$(resolve_path_for_scan "$file")
  [[ -z "$target" ]] && continue

  scan_one "$file" "$target" "word" "$WORDS_RE" "-i"
  scan_one "$file" "$target" "pattern" "$PATTERNS_RE" ""
done

# ------------------------------------------------------------------
# 结果
# ------------------------------------------------------------------
if [[ $hit_count -eq 0 ]]; then
  if [[ "$MODE" != "staged" ]]; then
    echo "[敏感词扫描] 通过：已扫描 ${#TARGET_FILES[@]} 个文件，0 命中。"
  fi
  exit 0
fi

cat >&2 <<EOF

[敏感词拦截] 发现 ${hit_count} 处可能的敏感信息：

${hit_output}
如需临时跳过本次检查（不推荐）：
  SKIP_SENSITIVE_SCAN=1 git commit ...

或在 .cursor/skills/.sensitive-words.yaml 的 allowlist 里添加合法片段。

EOF
exit 1
