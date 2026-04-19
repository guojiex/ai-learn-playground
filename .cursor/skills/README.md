# .cursor/skills/

这个目录放 **Cursor Agent Skills**：可被 agent 自动发现并调用的结构化能力包。每个 skill 是一个带 `SKILL.md` 的子目录。

## 当前 skills

| 名称 | 说明 |
|------|------|
| [processing-pptx/](processing-pptx/SKILL.md) | 读取 / 创建 / 修改 PowerPoint 文件（python-pptx） |
| [office-politics-combat/](office-politics-combat/SKILL.md) | 互联网公司职场政治对话顾问：混合顾问模式，三档话术 + 钩子 + 反击闭环 |

## 敏感词保护（pre-commit 钩子）

因为 `office-politics-combat` 这类 skill 容易在案例里夹带真实姓名 / 公司名，这里配了一个 Git pre-commit 钩子在 commit 前自动扫描本目录下的 markdown。

### 启用（clone 下来后执行一次即可）

```bash
bash .cursor/skills/scripts/install-hook.sh
```

脚本会把 [scripts/check-sensitive.sh](scripts/check-sensitive.sh) 软链到 `.git/hooks/pre-commit`。

### 手动扫描

任何时候都可以手动跑：

```bash
# 扫全目录（工作区内容）
bash .cursor/skills/scripts/check-sensitive.sh

# 只扫某个文件
bash .cursor/skills/scripts/check-sensitive.sh .cursor/skills/office-politics-combat/SKILL.md
```

### 紧急跳过

```bash
SKIP_SENSITIVE_SCAN=1 git commit ...
```

不推荐。真要用说明你触犯了规则但有理由，建议改走 allowlist。

### 维护敏感词

唯一配置文件：[.sensitive-words.yaml](.sensitive-words.yaml)

三段：

- `words` —— 精确词（大小写不敏感、单词边界匹配）
- `patterns` —— 扩展正则（ERE，给 `grep -E` 用）
- `allowlist` —— 行内片段白名单，命中行若包含 allowlist 任一条目则放行

修改后自检：

```bash
bash .cursor/skills/scripts/check-sensitive.sh
```

## 脱敏约定

所有 skill 文件（尤其是 `cases/` 里的案例）必须使用以下代称，**不得出现真实姓名 / 公司名 / 内部代号**：

| 角色 | 代称 |
|------|------|
| 用户自己 | `Me` |
| 直属老板 | `Boss` |
| 老板的老板 | `BigBoss` |
| QA leader（白手套角色）| `QA_Gatekeeper` |
| QA 执行同学 | `QA_Member` |
| 平级同事 | `Peer` |
| HR / 越级对象 | `HR` / `Skip` |
| 内部 AI 战略 / 产品代号 | `<AI 战略代号>` |
| 内部工单号 / 文档链接 | 直接省略或占位为 `<工单号>` / `<文档链接>` |

pre-commit 钩子是这套约定的最后一道防线，不是第一道。写的时候就注意代称，比事后改省事。
