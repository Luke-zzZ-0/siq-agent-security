# N01 开发完成与 Linux 验收

**状态：N01 开发完成，满足任务书要求的组件验证及至少真实 Linux 二进制验收门槛。** Windows/macOS 原生验证继续列入 N07/N09，不计为已通过；本结论不是个人产品整体完成或正式发布许可。

本批在 `/tmp/siq-personal-closure` 的 `glm/personal-closure-20260913-161000` 分支实现，基线 `ff99317450784c9563f5b6a2308c98e8262df98d`。包含前一轮已验证的入口修复，本轮进一步实现协议、迁移、回退预检和恢复体验。所有修改仍未提交、未推送，未改旧 IDE 工作树的分支/索引、真实用户状态或系统服务。

## 1. 已交付行为

| 任务书 N01 要求 | 实现与验收 |
| --- | --- |
| 所有持久化入口的格式保护 | `stateformat` 负责严格元数据检查，`statefs` 在业务文件读写前检查所有带标记/屏障的祖先。覆盖 state、signing、receipt、pending、intent、runtimeidentity、skillimport/install、rawcontent、适配器、制品与 CLI。嵌套标记不能遮蔽外层不兼容状态。具体清单见 [入口覆盖](write-entry-coverage.md) |
| reader/writer、格式版本、实例绑定 | 新 `state-format/v2` 添加 min_reader/min_writer、目录摘要和实例 ID；独立读/写版本为 2。未知 schema、重复键、尾随 JSON、类型/预算错误、未来读写要求、静态符号链接和错实例拒绝 |
| 新旧状态识别 | 全新 init 发布 v2；已有密钥/授权等历史对象即便没有配置和实例也保留 v1，不绕过显式迁移。已知无标记格式族继续支持；未知非空目录沿用前一轮有界拒绝规则 |
| 完整备份、迁移与中断恢复 | `state-migrate --confirm` 仅支持已初始化 v1/已识别无标记 → v2。主锁与维护锁排他；不可变计划绑定源摘要、实例、目标及递归清单；逐项备份校验；提交前再校验；完成点落盘后归档并清理活动屏障。每个已定义检查点及每个备份条目后注入失败均可恢复 |
| 不复活权限、不重签历史 | 此次转换的是兼容元数据，业务格式保留原字节。真实旧程序创建并撤销的签名 Grant 在迁移后保持原修订，新程序读回仍为 revoked；没有恢复旧批准或重签历史 |
| 升级/回退在副作用前预检 | 新 `skill-manifest/v3` 签名覆盖 state_compatibility；候选实际摘要与声明、当前状态范围一起验证。v2 状态拒绝无声明的旧清单；在 staging、恢复旧二进制、停止/切换服务前检查并在执行前复验。既有健康读回与版本核验继续保留 |
| 旧程序拒写的真实证据 | 使用上一轮审查后的真实 v1 防护二进制，先创建合法 v1 状态，再由新二进制迁移；旧程序 init/pubkey/serve/grant 全部拒绝，文件内容、权限和清单不变。非 mock 版本号判断 |
| 可执行的恢复指引 | 新 `state-status` 只读诊断；CLI 不兼容错误与 UI HTTP 503 提示保留目录、检查状态及恢复中断迁移。离线 UI 也说明启动出现兼容错误时的检查命令，不输出后端私有路径 |

规格见 [N01 状态协议与迁移实施规格](../../../n01-state-protocol-design-20260913.md)；合同在 `packages/contracts/`：状态格式 v1/v2、迁移计划/结果、诊断结果、发行清单 v3，并有 Go 输出样例与 Python 校验。

## 2. 关键修复和设计取舍

- 状态包依赖签名/回执等模块，因此把基础检查放入仅标准库的独立包，避免为了覆盖写入口引入循环依赖。状态包继续负责旧目录识别，底层封装不把普通目录凭空认证为 SIQ 状态。
- 迁移没有调用方可指定的步骤、备份路径或目标格式；固定支持窗口和固定路径，未实现格式保持拒绝。备份保存嵌套数据、SHA256、文件大小和权限，不像原实现只复制顶层文件。
- 计划和检查点均不可变；不再尝试用“相同 kind 即兼容”的回调更新进度。完成备份后才能切换标记；失败 rename 后缺失标记只有完整备份证明及精确 prepared target 同时存在才允许恢复。
- 未发布的临时文件位于私有 journal/tmp，重启保留，不按名字删除未知文件。迁移完成后将计划归档到 `state-migration-v2/plan.json`，普通请求无需反复读取整份计划。
- 迁移只备份并改变兼容协议元数据，不把旧备份覆盖到已有新业务数据。清理已完成屏障时不把迁移完成后的正常新业务写入误判为需要回滚。
- CodeBuddy 钩子继续返回结构化拒绝，避免单纯退出 1 被宿主视为非阻断错误。已有安全 Git 拒绝、签名权限和原文保护逻辑保留。

## 3. 验证结果

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| Go 全量 | 39 个有测试包通过，6 个包无测试 | [go-final.log](go-final.log) |
| Go race | 全模块通过；最后初始化边界改动另复验 state/CLI | [race.log](race.log)、[final-race.log](final-race.log) |
| go vet / gofmt / diff | 通过 | [vet.log](vet.log)、[gofmt.log](gofmt.log)、[diff-check.log](diff-check.log) |
| Python 合同 | 206 passed | [python.log](python.log) |
| Ruff | 修改的合同测试文件通过 | [ruff.log](ruff.log) |
| Web | 74 项通过；TypeScript 与本地 embed 构建通过 | [web-test.log](web-test.log)、[web-build.log](web-build.log) |
| 真实 Linux 双二进制 | v1→v2、完整备份、已撤销 Grant 保持、旧程序四入口零写入拒绝、重复迁移/只读诊断 | [native.json](native.json)、[脚本](native.py) |
| 真实 Linux 服务 | 指定目录启动、签名停止、stop-request、stop 四项通过 | [native-service.log](native-service.log) |
| 四目标构建 | linux amd64/arm64、darwin arm64、windows amd64 全通过，CGO_ENABLED=0 | [构建与 SHA256](build-results.json) |

原始失败证据仍见前一轮 [审查报告](../ornith-n01-review-fixes-20260913-182231/report.md)。本批记录不覆盖、冒用旧日志；源码摘要见 [改动身份](changed-source-sha256.json)。基线 HEAD 不是包含本轮改动的提交。

## 4. 使用与恢复

以下命令是交付能力说明。本次演练只在临时目录执行，未迁移用户真实状态。

```bash
siq-agent-security state-status
# 先正常停止本实例，再确认对所选状态进行私有备份和迁移：
siq-agent-security state-migrate --confirm
# 完成后启动原实例：
siq-agent-security serve
```

需要选择其他状态目录时设置 `SIQ_AGENT_SECURITY_STATE_DIR`；格式绑定实际原目录和 `local-instance.json`，不能把目录搬走后伪造新实例直接使用。活动计划在 `logs/migration-plan.json`；完整备份、归档计划和完成点在 `state-migration-v2/`。同一命令恢复中断步骤，完成后重复调用返回 up_to_date。源数据/备份/计划漂移时保留现场拒绝；不要删除标记或回放旧 Grant 来绕过检查。

备份清单只存路径、模式、大小与摘要；实际备份包含本机密钥等原数据，所以目录保持私有。本批不提供自动覆盖业务状态的“恢复旧快照”按钮，避免把升级回退变成权限回退。

## 5. 诚实边界与后续

1. 旧二进制演练对象是上一轮已构建的兼容感知 v1 防护版本（SHA256 固定），不是任意历史 main 或正式发行版本。历史未实现检查的程序、同 UID 恶意进程和直接删除标记不在绝对拒写保证内。正式发行签名与实际制品能力仍由 N07 的发布验证承担。
2. 原生平台证据仅 Linux arm64。其他三个制品是构建证据；Windows/macOS 的路径、文件系统、系统服务和断电行为继续放入 N07/N09 实机矩阵。Linux 执行文件/目录同步，但没有以真实断电实验替代可控故障注入。
3. 当前迁移预算：10,000 条目、单文件 128 MiB、合计 2 GiB。只接收普通文件和真实目录，拒绝 symlink、特殊文件及特殊权限位。备份覆盖文件字节/POSIX 模式，不宣称复制 OS ACL/xattr 或支持任意重定位。
4. `state-status` 是兼容诊断，不是对所有业务对象签名或未来备份损坏的全面扫描。业务模块仍各自验签和验证状态，迁移不会把内容重打成可信证据。
5. 后续按任务书优先推进 N02 安全 Git 和 N04 平台能力核验；N03/N07 可依赖本协议继续。N09 未完成前不进入团队阶段。N00 的最新主线/旧工作树全量差距复核仍单列，不因本轮 N01 完成而自动勾选。
