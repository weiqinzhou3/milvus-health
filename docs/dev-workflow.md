# milvus-health Development Workflow

## Roles and responsibilities

- Codex: 小步开发 + push + 说明改动。
- Claude Code: 测试与验证 + push。
- DBA: 真实环境验证。
- ChatGPT: 架构/评审/任务拆分。

## Definition of Done

一次迭代只有在以下条件全部满足时才算完成：

- 代码改动完成。
- 测试通过。
- push 到 GitHub。
- 更新 `docs/project-status.md`。
- 返回 branch / commit SHA / 改动文件 / 验收命令。

## Development rules

- 不允许一个 prompt 做完整项目。
- 必须按小迭代推进。
- 一轮只做一个小目标。
- 不允许越范围开发。

## Recommended workflow

1. 建分支。
2. 编码。
3. 测试。
4. 更新文档。
5. push。
6. 回报结果。

## Audit 分支与正式修复分支规则

在本项目中，Codex 与 Claude Code 的职责不同：

### 1. Codex
- 负责主实现与正式修复
- 其分支通常作为主交付分支
- 若通过评审，可直接合并到 `main`

### 2. Claude Code
- 负责测试、审计、覆盖率补强、golden 校验与小范围契约性修复
- 若 Claude Code 修改了仓库文件（包括测试、golden、文档），也必须 commit 并 push 到 GitHub
- 其分支默认视为 **audit 分支**，用于：
  - 提供审计证据
  - 暴露 coverage gap
  - 让 reviewer 与 Codex 可见具体改动
- audit 分支 **默认不直接合并到 `main`**

### 3. 推荐合并策略
- 若 audit 分支中的有效内容已经被 Codex 吸收到正式 fix 分支，则：
  - 只合并正式 fix 分支
  - audit 分支不单独合并
- 只有在 audit 分支中存在未被吸收、但 reviewer 明确认可应直接进入主线的有效修复时，才考虑直接合并 audit 分支

### 4. 判断规则
- **改了文件**：必须 push
- **是否合并**：看该分支是否是“正式交付分支”，而不是看它有没有改代码

### 5. 当前项目默认原则
- `fix/*`、`feat/*`：优先作为正式交付分支
- `audit/*`：优先作为审计记录分支，不单独合并