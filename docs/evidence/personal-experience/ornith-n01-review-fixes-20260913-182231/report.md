# Ornith N01 阶段深度审查与修复

时间：2026-09-13，Asia/Shanghai。结论：**原 N01 交付不通过完整验收；本轮修复后的状态格式入口防护通过组件和 Linux 二进制验证，N01 仍为 doing。**

## 1. 范围和基线

- 审查用户已暂停的 Claude Code 工作树 `/tmp/siq-personal-closure`，分支 `glm/personal-closure-20260913-161000`，基线 `ff99317450784c9563f5b6a2308c98e8262df98d`。
- 原增量为 N00 基线材料、N01 `compatibility.go`/测试、`commit.go` 替换原语和 `main.go` serve 接入。未发现本工作树已经交付 N02–N09/T01–T06 的增量证据。
- 本轮没有切换或覆盖旧 IDE 工作树，也没有提交、推送、合并或发布。此基线 SHA 不是包含本轮修复的提交，修复仍在工作区。
- 原实现、原报告和原始失败复现已保存在 `/home/maoyd/siq/.review-snapshots/ornith-n01-20260913`；摘要见 [快照清单](before-snapshot-manifest.json)。不把原先通过的测试日志冒充修复后证据。

## 2. 已确认问题

| 严重性 | 问题与实际影响 | 修复 |
| --- | --- | --- |
| P1 | `state.Open` 在格式检查前创建核心目录；未来格式被接受，测试目录从 1 项变为 18 项 | Open 在任何 MkdirAll 前只读检查，并预检已有核心目录是否为真实目录 |
| P1 | `AcquireWriter` 不检查格式，只有 serve 后置检查；初始化、CLI、维护子锁等可绕过 | CLI 分派、实际 serve 目录、根 Writer 和显式作用域 Writer 前置检查；取得锁后复验 |
| P1 | 重复 `format_version` 最后值胜出、尾随第二个 JSON 对象被接受 | 完整单对象、重复键/大小写别名/未知字段/缺失/null/类型/预算/UTF-8/时间校验，Go 与 schema 共用样例 |
| P1 | `backupForMigration` 没有备份嵌套 Grant/回执；不能作为整树恢复点 | 删除未经实现和验收的自动迁移/备份路径，显式旧格式拒绝并保留原数据 |
| P1 | `WriteMigrationPlan` 使用不可变发布但兼容回调忽略更新内容，Committed 从 -1 更新后磁盘仍为 -1 | 删除不可靠的迁移重放；现存 migration-plan 拒绝自动恢复，不发布伪成功新格式 |
| P2 | 损坏检查修改值副本，返回 Status 为空 | 用显式返回值统一返回 corrupt/future 等状态与稳定错误 |
| P1/P2 | 文档把占位步骤、未完整备份和未持久化进度描述为“可重启迁移完成”，覆盖清单把仅 serve 检查说成全入口 | 原材料标记 superseded，另建覆盖表和真实验证记录，N01 保持 doing |

前六项均在原代码上复现失败，见 [原始失败日志](reproduction-before.log) 与 [复现源码](reproduction-before_test.go.txt)。审查还发现原步骤实现没有实际转换逻辑、路径/计划缺少可信绑定，以及审计失败只告警等问题，不能在这些基础上继续宣称具备迁移保障。P1 表示本功能验收阻塞，不代表已证明可跨 OS 安全边界利用。

## 3. 修复后的行为

1. 未来格式、显式不支持的旧格式、损坏或含歧义的标记在访问/写入前拒绝。检查本身不建立目录、配置、密钥或锁，不输出标记原文。
2. 空目录允许初始化；已知旧版核心目录结构、合法 config/signing seed，以及既有独立子存储目录保留无版本兼容。未知非空目录拒绝。目录识别不替代业务对象验签。
3. 只有显式 init 在初始化验证通过后排他增加格式标记。正常 Open/serve 不自动重打标记，不重签或覆盖历史权限与回执。
4. 维护锁显式传入真实状态根，防止根目录刚好叫 service-control 时被误认为子目录；拒绝未知 scope 和路径穿越。产品已有服务、适配器和制品锁保持原来的独立锁语义。
5. 核心 Store 写入、Server 构造、HTTP 分派和原文清理增加复验；服务无法安全读取状态时返回 503。CodeBuddy hook 保留结构化 deny，避免退出 1 被宿主当成非阻断失败。
6. 调整少量旧测试 fixture：用于模拟已初始化状态的目录先创建真实核心结构；只读 preview 仍对全新空目录断言零写入，不放松原有安全断言。

规格见 `docs/agentshield-dev-spec-v1.md` §2.3.1，合同见 `packages/contracts/local-state-format.v1.schema.json`，写入口清单见 [修订覆盖表](../n01-state-compatibility/write-entry-coverage.md)。

## 4. 本轮验证

| 验证 | 实际结果 | 证据 |
| --- | --- | --- |
| go test ./... | 37 个有测试包通过；6 个包无测试 | [日志](go-all.log) |
| go vet ./... / gofmt / git diff --check | 通过；格式/差异检查无输出 | [vet](vet.log)、[gofmt](gofmt.log)、[diff](diff-check.log) |
| state/server/CLI race | 三包通过 | [日志](race.log) |
| Python 合同 | 201 项通过；含新增状态格式合同 | [日志](contracts.log) |
| Ruff | 修改的合同测试文件通过 | [日志](ruff.log) |
| Linux 原生服务 | 指定目录、签名停止、stop-request、stop 四项通过 | [日志](native-service.log) |
| Linux 原生格式 | 7 类拒绝 × init/pubkey/serve 共 21 次零写入；另 3 项初始化/无标记兼容验证 | [结果](native-format.json)、[演练脚本](native-format-drill.py) |
| 四目标构建 | CGO_ENABLED=0，linux amd64/arm64、darwin arm64、windows amd64 全通过 | [构建结果与 SHA256](build-results.json) |

CLI 子进程回归覆盖多个管理入口，状态核心测试覆盖持有 Store 后标记改变的 10 个写边界、4 类维护锁及拒绝前后路径/模式/内容一致性。原有业务测试继续通过。这些测试证明列出的行为，不等于已证明所有历史状态和所有 OS 场景。

## 5. N01 尚未完成，下一执行者必须保留的边界

- 分离最低 reader/writer 版本和状态实例绑定；当前四字段标记只是已知格式族的附加元数据。ProgramVersion 尚未接入正式发行身份，写为 unversioned，不伪造来源。
- 对每种独立持久化模块形成完整入口枚举和直接调用测试。当前 CLI/HTTP 入口及核心 Store 复验，不代表所有内存持有子存储都有统一事务边界。
- 实现有明确支持窗口的真实旧格式转换器、完整备份清单、摘要/归属验证、受锁迁移日志、崩溃恢复和每一故障点演练。没有 converter 时保持拒绝，不恢复空壳 ApplyMigration。
- 将发行兼容声明、升级预检、切换和回退绑定同一协议；以实际新旧两个受支持二进制证明旧版本拒写，不复活 revoked/expired 权限。
- 补齐 UI 可执行恢复指引。当前稳定错误与 HTTP 503 不是完整用户恢复流程。
- Windows/macOS 原生验证尚未进行；交叉编译不等于原生通过。静态符号链接检查不是任意同 UID 并发篡改隔离。当前拒绝祖先符号链接，macOS 等原生路径习惯需要在实际安装根和临时目录中验证。
- N00 的旧工作区全量差异复核、最新远端差距和发布前冻结仍需单列证据；本轮只深审本工作树 N01 增量，不把此前主线已合并等同本轮已验收全仓库。

下一批继续 N01 的协议设计与入口覆盖；N02/N04 可按任务书做无依赖研究。N09 未验收前不进入团队实施。不要将这次修复表述为整体开发已完成，也不按新增文件数或台账批次估算百分比。
