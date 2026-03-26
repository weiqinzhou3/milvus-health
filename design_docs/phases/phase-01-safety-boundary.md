## Milvus Health Phase 01  
## Safety Boundary & Beta Positioning

### 1. Phase Goal
将 `milvus-health` 从“存在误伤客户环境风险的早期 CLI”收口为“**默认安全、风险边界明确、适合工程师陪同使用的 beta 工具**”。

本阶段优先解决 reviewer 已指出的最高风险问题：

- CLI/help 文案宣称 `read-only`，但启用 RW probe 后会真实建库、建集合、写入、删库
- 运行开始时会按前缀清理旧测试库，即使 `cleanup=false` 也会发生
- 这会造成客户误用、前缀误配、并发执行时的真实删库风险

---

### 2. Why This Phase Exists
当前项目最大的问题不是“功能不够多”，而是**行为边界和产品承诺不一致**。

只要这个问题不先收口，后面做：

- spec 修订
- gate 建设
- review routine
- 对外 beta 说明

都会失去基础。

---

### 3. In Scope
本阶段只做“风险边界收口”，范围如下：

#### 3.1 产品定位与文案
- 将当前工具定位明确为：
  - **engineer-assisted beta tool**
  - 非“客户自助安全巡检工具”
- 修正 CLI 帮助、README、Quickstart、示例配置中的相关表述
- 清除或替换所有会误导用户认为 `check` 一定是只读的文案

#### 3.2 RW probe 风险开关
- RW probe 必须默认关闭
- 只有显式开启时才允许进入 RW 流程
- 开启方式必须是**危险操作显式确认语义**
- 必须允许使用者通过配置和 CLI 明确看出：
  - 当前是否启用了 RW
  - 当前运行会不会写 Milvus
  - 当前是否允许 cleanup

#### 3.3 清理行为止血
- 禁止“启动即按前缀扫旧测试库”的隐式行为
- cleanup 必须受显式开关控制
- cleanup=false 时，不允许发生任何删除测试库动作
- 对已有测试库的处理策略必须重新定义为：
  - 要么显式 cleanup
  - 要么显式 fail
  - 要么跳过并报告
- 不能再有“即使 cleanup=false 也先删一遍”的路径

#### 3.4 风险告知与输出可见性
- 在执行前或输出结果中，必须明确暴露本次运行模式：
  - read-only
  - RW enabled
  - cleanup enabled / disabled
- 结果输出中要能让工程师快速判断是否跑了危险模式

---

### 4. Out of Scope
本阶段不做以下事情：

- 不扩新 probe 类型
- 不实现 standby 闭环
- 不补 metrics TLS/认证配置
- 不清理重复包树/死代码
- 不做大规模架构重构
- 不改变现有渲染格式本身的整体设计
- 不做 release 包装优化

---

### 5. Required Decisions
本阶段必须先做出以下决策，不能边写边漂：

#### 5.1 check 命令的产品语义
必须二选一：

**方案 A（推荐）**
- `check` 默认定义为 safe check
- RW probe 是危险扩展能力，必须显式开启

**方案 B**
- `check` 只是统一入口，不再宣称 read-only
- 但默认仍然只能执行 safe paths

我建议选 **方案 A**。
因为 reviewer 已经明确指出当前最大问题就是“help 和行为冲突”。

#### 5.2 cleanup 冲突策略
当存在历史测试库时，必须固定一种行为：

- 显式 cleanup 才删
- 未显式 cleanup 时直接失败并报清楚
- 或未显式 cleanup 时跳过 RW 并报 WARN

我建议优先采用：

**未显式 cleanup 时，不删库，直接 fail fast 并提示人工处理**  
这样最保守，也最利于客户环境安全。

---

### 6. Deliverables
本阶段交付物应包括：

1. 一份更新后的 phase 文档
2. 一次性修正文档：
   - README
   - Quickstart
   - examples/config.example.yaml
   - CLI help 文案
3. 一组 RW 风险保护测试
4. 一份“危险行为说明”小节
5. 一份 beta 定位说明小节

---

### 7. Test Requirements
本阶段测试必须围绕“默认安全”和“危险模式显式化”来设计。

#### 7.1 必做单元/集成测试
- 默认配置运行时，不触发 RW probe
- 默认配置运行时，不触发任何 cleanup
- 显式开启 RW 后，才允许进入 RW 逻辑
- `cleanup=false` 时，绝不能发生删库
- 历史测试库存在但未开启 cleanup 时，行为符合选定策略
- CLI help / config / 实际行为一致

#### 7.2 必做负向测试
- 配置误以为 read-only，但显式打开 RW 时，应清楚暴露危险模式
- cleanup=false + 存在旧测试库，不得隐式删除
- 未显式开启 RW，但传入部分 RW 相关字段时，不得误进入写路径

#### 7.3 输出验证
- text 输出至少应能看出：
  - probe mode
  - RW enabled / disabled
  - cleanup enabled / disabled
- json 输出也必须包含同等语义字段或等价信息

---

### 8. Acceptance Criteria
本阶段完成的标准不是“代码能跑”，而是以下条件全部满足：

1. 默认执行 `check` 时不会发生真实写入和删除
2. 所有 RW 行为都需要显式危险开关
3. cleanup=false 时绝不删库
4. 文档、CLI help、示例配置与真实行为一致
5. 使用者从输出结果中能一眼看出当前是否运行了危险模式
6. 本阶段改动不引入新的产品语义漂移

---

### 9. Review Checklist
本阶段 review 只看这些：

#### Design Review
- `check` 的产品语义是否唯一
- RW 和 cleanup 是否都变成显式危险路径
- 默认行为是否绝对安全

#### Implementation Review
- 是否还存在隐式 cleanup
- 是否还存在 help/README 与行为冲突
- 是否存在绕过危险开关进入 RW 的路径

#### Test Review
- 是否覆盖默认安全路径
- 是否覆盖 cleanup 负向路径
- 是否验证输出中可见的运行模式信息

---

### 10. Exit Rule
只有当以下问题都收口后，本 phase 才算结束：

- “read-only”与真实行为冲突被修正
- RW 默认关闭
- 隐式扫库式 cleanup 被移除
- 默认路径安全
- 风险模式清晰可见
