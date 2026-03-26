# Milvus Health Phase 02
# Review & Acceptance Checklist

本清单用于 Phase 02 的正式审查与放行。

适用对象：
- 你（phase owner）
- Codex（提交者 / 自审）
- Claude / 其他 agent（独立 reviewer）
- 我（项目总协调 / 最终辅助判断）

本阶段重点不是“功能多不多”，而是：
- 配置契约是否可信
- 输出契约是否可信
- validate 与 runtime 是否一致

---

## 1. Phase 02 审查目标
本阶段必须回答清楚以下问题：

1. YAML 配置写错字段时，程序是否会 fail fast？
2. `output.format` / `output.detail` 的 YAML 配置是否真的生效？
3. CLI、YAML、默认值三者优先级是否唯一且稳定？
4. validate 通过的配置，运行时是否真正遵守其语义？
5. 输出中的失败/跳过/不可执行状态是否清楚，不再出现关键空白态？

如果这 5 个问题不能同时回答“是”，Phase 02 不应放行。

---

## 2. 设计 Review Checklist
先审设计和范围，不先看“写了多少代码”。

### 2.1 范围控制
- [ ] 本轮目标明确聚焦 Phase 02
- [ ] 没有混入 Phase 03（standby / metrics TLS）内容
- [ ] 没有混入 Phase 05（重复代码/死代码清理）内容
- [ ] 没有借机做大规模 config/render 重写

### 2.2 配置契约设计
- [ ] unknown field 策略已明确为 fail fast
- [ ] merged config 的 source of truth 已明确
- [ ] CLI > YAML > default 优先级已明确
- [ ] validate 的执行对象是最终 merged config

### 2.3 输出契约设计
- [ ] `output.format` 语义明确
- [ ] `output.detail` 语义明确
- [ ] 渲染器消费的是最终有效配置，不是裸 CLI opts
- [ ] 失败/跳过/不可执行状态有明确输出语义

### 2.4 决策质量
- [ ] 采用的是最小闭环方案，而不是大重构
- [ ] 所有设计决策都能被测试证明

---

## 3. 实现 Review Checklist

### 3.1 配置严格校验
- [ ] YAML 未知字段会直接报错
- [ ] 不再 silently ignore unknown fields
- [ ] 错误信息对用户可理解
- [ ] example config 中字段均能被程序识别

### 3.2 配置合并与优先级
- [ ] 默认值加载正常
- [ ] YAML 能覆盖默认值
- [ ] CLI 能覆盖 YAML
- [ ] 没有出现多个入口使用不同优先级规则

### 3.3 输出配置生效
- [ ] `output.format` 在 YAML 中配置后真正影响输出
- [ ] `output.detail` 在 YAML 中配置后真正影响输出
- [ ] CLI 显式参数能覆盖 YAML
- [ ] text/json 行为与文档一致

### 3.4 validate/runtime 一致性
- [ ] validate pass 的配置运行时语义一致
- [ ] 不支持的语义被 validate 拒绝或被明确实现
- [ ] 至少已检查 `probe.read.min_success_targets`
- [ ] 没有继续保留明显的 validate/runtime mismatch

### 3.5 失败语义与可见性
- [ ] `business_read` 连接失败时状态明确
- [ ] skip / fail / unavailable / not-run 不再混成空白
- [ ] 输出不会让用户误判为成功或无事发生

### 3.6 文档与样例同步
- [ ] README 已同步
- [ ] config example 已同步
- [ ] output examples 已同步
- [ ] project-status / release note（如涉及）已同步

---

## 4. 测试 Review Checklist

### 4.1 配置解析测试
- [ ] 已知字段解析测试存在
- [ ] 未知字段负向测试存在
- [ ] 默认值测试存在
- [ ] YAML 覆盖默认值测试存在
- [ ] CLI 覆盖 YAML 测试存在

### 4.2 输出契约测试
- [ ] YAML 驱动 text/json 输出测试存在
- [ ] YAML 驱动 detail 输出测试存在
- [ ] CLI 覆盖 YAML 输出测试存在
- [ ] text golden tests 存在
- [ ] json golden tests 存在

### 4.3 validate/runtime 一致性测试
- [ ] 边界值测试存在
- [ ] validate 通过后的运行语义测试存在
- [ ] 不支持语义拒绝测试存在

### 4.4 失败语义测试
- [ ] `business_read` 失败状态测试存在
- [ ] skip/fail/unavailable 等状态区分测试存在
- [ ] 不再只是 happy path 测试

### 4.5 回归测试
- [ ] 未破坏 Phase 01 的 safe/dangerous 输出语义
- [ ] 未破坏 rw/cleanup 可见性
- [ ] 现有基础 build/test 能通过

---

## 5. 证据要求
不能只接受“我改了这些”的叙述，必须要有证据。

审查时应至少拿到：

- [ ] git diff
- [ ] 改动文件列表
- [ ] `go test ./...` 结果
- [ ] `go build ./...` 结果
- [ ] 新增/修改测试清单
- [ ] 更新后的 example config
- [ ] 更新后的 output examples
- [ ] 提交者自审报告

如果缺少以上关键证据，不建议直接放行。

---

## 6. 人工验收动作
即使代码审查通过，phase owner 仍应做最小人工验收。

### 6.1 命令级验收
至少做：
- [ ] `go test ./...`
- [ ] `go build ./...`
- [ ] 使用默认配置跑一次
- [ ] 使用 YAML 指定 `output.format=json` 跑一次
- [ ] 使用 YAML 指定 `output.detail=true` 跑一次
- [ ] 使用错误字段配置跑一次，确认 fail fast

### 6.2 结果级验收
人工确认：
- [ ] 输出格式确实由 YAML 驱动
- [ ] detail 确实由 YAML 驱动
- [ ] 错误字段不会被悄悄忽略
- [ ] 失败状态不是空白态

### 6.3 文档级验收
人工肉眼确认：
- [ ] README 与实现一致
- [ ] examples/config.example.yaml 与实现一致
- [ ] 示例输出与实现一致

---

## 7. 结论模板
审查结论只能是以下三种之一：

### PASS
适用条件：
- Phase 02 的核心目标全部完成
- 无阻塞问题
- 可进入下一 phase

### PASS WITH FOLLOW-UPS
适用条件：
- 核心目标完成
- 仍有非阻塞余项
- 这些余项已明确归属下一 phase

### FAIL
适用条件：
- 配置契约或输出契约仍不可信
- 未知字段仍能静默通过
- YAML 仍无法真正驱动输出
- validate/runtime mismatch 仍是核心问题

---

## 8. 常见不通过情形
以下任一情形出现，都不应直接放行：

- [ ] 只更新了代码，没更新 example / 文档 / output samples
- [ ] 只做 happy path 测试，没有负向测试
- [ ] YAML 中 `output.format` / `output.detail` 仍不生效
- [ ] CLI、YAML、default 的优先级在不同路径不一致
- [ ] 未知字段仍被 silently ignore
- [ ] validate/runtime mismatch 只是被记录，没有被修复或收紧
- [ ] 提交者说“应该没问题”，但没有给证据

---

## 9. 审查结论记录模板
建议每次 Phase 02 审查都用下面这个最小模板记录：

```md
# Phase 02 Review Result

## Conclusion
- PASS / PASS WITH FOLLOW-UPS / FAIL

## What is accepted
- 

## Blocking issues
- 

## Non-blocking follow-ups
- 

## Evidence reviewed
- git diff:
- changed files:
- tests:
- examples:
- docs:

## Next step
- 
```

---

## 10. 放行规则
只有当以下条件同时满足，才建议放行 Phase 02：

- [ ] strict YAML validation 已生效
- [ ] `output.format` / `output.detail` YAML 配置已真正生效
- [ ] CLI > YAML > default 已固定
- [ ] validate/runtime 一致性达到可接受水平
- [ ] 失败语义不再关键空白
- [ ] 文档 / example / output sample / 实现再次对齐

若未同时满足，请不要进入下一 phase。
