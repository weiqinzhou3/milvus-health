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
