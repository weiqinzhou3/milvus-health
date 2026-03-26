## Milvus Health Phase 02  
## Config Contract & Output Contract

### 1. Phase Goal
建立 `milvus-health` 的**配置契约**和**输出契约**，解决“配置写了但不生效”“未知字段被静默忽略”“validate 通过但运行语义不一致”的问题。

本阶段优先对应 reviewer 已指出的问题：

- `output.format` / `output.detail` 配置值实际上不生效，只有 CLI 参数显式传入才生效
- YAML 对未知字段静默忽略
- 某些配置项 validate 能过，但运行时并不遵守其语义  
  例如 `probe.read.min_success_targets=0`

---

### 2. Why This Phase Exists
对于 CLI 运维工具来说，**配置是控制面本身**。

如果出现：

- 配置写了不生效
- 文档声明字段存在，程序却静默忽略
- validate 通过但运行逻辑不认

那这个工具即使功能再多，也不具备“工程可控性”。

所以本阶段的目标不是“加更多配置项”，而是把 **配置=契约** 这件事真正立住。

---

### 3. In Scope
#### 3.1 严格 YAML 校验
- 配置解析必须开启严格字段校验
- 未知字段必须报错
- 不允许继续静默忽略未知配置

#### 3.2 配置优先级规则
明确并实现唯一优先级：

**CLI 显式参数 > YAML 配置 > 默认值**

需要覆盖至少这些字段：
- `output.format`
- `output.detail`
- 其他当前已有用户可见控制项

#### 3.3 输出契约修复
- 配置中的 `output.format` 必须真正影响输出格式
- 配置中的 `output.detail` 必须真正影响 detail 输出
- 渲染逻辑必须读取最终合并后的配置，而不是只读 CLI opts

#### 3.4 validate 与运行语义一致
- validate 允许的值，运行逻辑必须能正确遵守
- 若运行逻辑不支持某种语义，则 validate 不应放行
- `probe.read.min_success_targets` 之类的配置要实现：
  - 要么语义支持到位
  - 要么收紧校验规则

#### 3.5 文档同步
- 更新 spec / config example / help / output 示例
- 文档中声明可用的字段，必须能被程序识别并落地
- 不再允许“设计里有，程序没接通，且解析时还不报错”的情况

---

### 4. Out of Scope
本阶段不做以下事情：

- 不新增大批配置项
- 不处理 standby 闭环
- 不处理 metrics TLS/认证设计
- 不做重复包树清理
- 不做完整 release 流程建设
- 不重构全部渲染体系

---

### 5. Config Contract Definition
本阶段要明确写进文档的配置契约：

#### 5.1 Unknown Field Policy
- 未知字段：**报错并退出**
- 不允许 warning 后继续运行
- 不允许 silently ignore

#### 5.2 Merge Policy
最终运行配置由以下规则生成：

1. 先加载默认值
2. 再加载 YAML 配置覆盖默认值
3. 再用 CLI 显式参数覆盖最终值

#### 5.3 Validation Policy
- validate 必须针对**最终合并后的配置**执行
- validate 的通过结果必须与运行语义一致
- 不允许存在“validate pass but runtime semantic mismatch”

---

### 6. Output Contract Definition
至少要把以下输出契约固定下来：

#### 6.1 format contract
- `text`：文本输出
- `json`：结构化 JSON 输出

若有其他格式，必须在文档中明确；没有就不要写模糊描述。

#### 6.2 detail contract
- `detail=false`：输出摘要级信息
- `detail=true`：输出扩展细节
- text/json 在 detail 语义上应各自清楚，不允许“只有 CLI 时生效、配置不生效”

#### 6.3 source of truth
渲染器必须只消费“最终有效配置”，不能再直接绕过配置合并层读原始 CLI opts。

---

### 7. Deliverables
本阶段交付物应包括：

1. 一份 phase 文档
2. 一份配置契约说明
3. 一份输出契约说明
4. 更新后的 config example
5. 一组配置解析/优先级/golden tests
6. 一组负向测试样例
7. 更新后的示例输出

---

### 8. Test Requirements

#### 8.1 配置解析测试
必须覆盖：
- 已知字段正常解析
- 未知字段直接报错
- 缺省配置按默认值生效
- YAML 可正确覆盖默认值
- CLI 可正确覆盖 YAML

#### 8.2 输出契约测试
必须覆盖：
- YAML 指定 `output.format=json`，不加 CLI 参数时输出 JSON
- YAML 指定 `output.detail=true`，不加 CLI 参数时 detail 生效
- CLI 覆盖 YAML 后，输出按 CLI 为准
- text/json 两种格式都要有 golden tests

#### 8.3 validate/runtime 一致性测试
必须覆盖：
- `probe.read.min_success_targets` 的边界值
- validate 通过后，运行逻辑必须符合其语义
- 若当前不支持某语义，则 validate 必须拒绝

#### 8.4 文档-实现一致性检查
至少检查：
- 示例配置中的字段都能被识别
- 文档列出的关键字段不会被 silently ignore

---

### 9. Acceptance Criteria
本阶段完成标准：

1. YAML 未知字段不再被静默忽略
2. `output.format` / `output.detail` 的配置值真正生效
3. CLI > YAML > default 的优先级明确且有测试证明
4. validate 与运行语义一致
5. config example、spec、help、示例输出与实现一致
6. 不再存在“字段在文档里写了，但程序不识别还不报错”的核心问题

---

### 10. Review Checklist

#### Design Review
- 配置优先级是否唯一
- unknown field 策略是否明确
- validate 是否以最终配置为准

#### Implementation Review
- 是否仍有路径直接读取 CLI opts 而绕过最终配置
- 是否真正启用了严格 YAML 校验
- 是否还存在 validate/runtime mismatch

#### Test Review
- 是否有未知字段负向测试
- 是否有 output.format / detail 的 golden tests
- 是否有配置优先级测试
- 是否覆盖关键边界值

---

### 11. Exit Rule
只有满足以下条件，本 phase 才算结束：

- 严格 YAML 校验生效
- 配置驱动输出恢复可信
- 配置优先级固定
- validate 与 runtime 行为一致
- 文档/示例配置/实现三者重新对齐
