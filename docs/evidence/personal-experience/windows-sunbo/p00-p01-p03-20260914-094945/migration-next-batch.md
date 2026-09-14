# N01 Windows 真实旧版到新版迁移：下一批准备

**本轮结论：在当前本地可达的完整 Git 历史和仓库归档材料中，尚未找到可给出 40 位源码 SHA、并能证明 reader=1 / writer=1 且支持活动迁移屏障的可信旧候选。旧候选 SHA 保持未确定，不能执行或计入正式 Windows 双二进制迁移验收。**

本轮只读取 Git 历史、N01 规格、台账、源码和归档脚本；未创建 worktree、未构建、未运行迁移、未修改 source。当前仓库不是 shallow clone；调查范围为当前本地 refs 可达历史，不排除维护方另有尚未纳入本仓库的旧源码快照。

## 1. 已核实的身份链

| 对象 | 完整身份 | 能证明什么 / 不能作为旧 v1 候选的原因 |
| --- | --- | --- |
| N01 开发前基线 | `ff99317450784c9563f5b6a2308c98e8262df98d` | 历史报告明确它是未提交修复的 base，不包含修复；该树没有 `internal/state/compatibility.go` 或 `internal/stateformat/format.go`，`git grep` 未找到状态标记/迁移屏障/ReaderVersion/WriterVersion 的实现。不能把基线 SHA 当修复源码 SHA。 |
| N01 合同提交 | `b2531edbc8e13af963d67449f4c2d03f498bd096` | 9ba20e8 的父提交，只新增六份合同文件；没有可重建的旧 v1 防护实现。 |
| 首次提交兼容实现 | `9ba20e87c57b8ead40f4df3e090301bea77da6fa` | `state/compatibility.go` 与 `stateformat/format.go` 的首次加入；该提交 `format.go:19–20` 已是 `ReaderVersion = 2` / `WriterVersion = 2`，并实现迁移屏障。它是真实 v2 源码，不是旧 v1。 |
| 历史证据归档提交 | `e2caa5e41ee151bf7c16ca3d76c6a750f7995521` | 将旧阶段报告、摘要和演练脚本纳入 Git；归档材料不等于归档了可构建的完整旧源码树。 |
| N01 合入 main | `0d4133f03ec23bb13af5765f3c731138e5595e7a` | 台账所述 PR #35 合并结果，已包含 v2；不能作为 reader/writer v1。 |
| 本次 Windows 新候选 | `ebc472f2e46aa7de837afe9d6a0ed422eef51cd0` | 已验证的新程序，reader/writer 2；Windows binary SHA-256 为 `4bc3f5ae95fd00aab528363d5d91e64145e0fd75257904a9f898323de68b6f74`。如下一批源码变化，须重新固定新候选与二进制，不能沿用本批身份。 |

历史 Linux 演练确实给出了旧二进制身份：

- `docs/evidence/personal-experience/n01-completion-20260913-190637/native.json` 的旧 Linux/arm64 SHA-256：`074f4fa94c3cba4d7acacec5c58a438e8bacfd77c183de3630f05afab1a36df4`。
- 同摘要可在上一轮 `ornith-n01-review-fixes-20260913-182231/build-results.json` 中交叉核对；该文件还列出 Windows/amd64 交叉编译 SHA-256：`9e54d1906ac88efecb23ffd05015d2e03bc6666004b68c3e55b42269a0af8acf`。本机未取得或执行该旧 Windows 二进制；构建结果记录不是原生验证。
- 旧轮 `verification.json:5–8` 明确 `base_head=ff993174...`、`uncommitted_changes=true`。旧轮 `report.md:54` 仍将独立最低 reader/writer 版本列为待实现，因此不能由 `format_version=1` 推导代码具有两个显式版本 1 常量。
- 完成轮 `verification.json:86` 给出维护方保存的 `old_binary_archive`，但只指向 Linux 二进制；`changed-source-sha256.json` 是部分改动文件摘要，不包含这些文件的完整旧字节，也不是完整源码树或可还原补丁。
- `native.py` 的旧程序对象来自该旧构建，而非通过改 `main.Version` 来伪造旧程序。这一历史证据继续保留其原来 Linux 范围；当前欠缺的是将旧对象可追溯地重建为 Windows 原生程序所需的源码身份。

## 2. 迁移屏障的源码与证据缺口

N01 规格 `docs/n01-state-protocol-design-20260913.md:19` 要求在完整转换期间，旧 v1 防护程序也拒绝 `logs/migration-plan.json` 活动屏障。仅在目标变成 v2 后因未来格式而拒绝，不能代替此要求。

当前新代码的可核对依据：

- `internal/stateformat/format.go:19–21`：reader/writer 2，固定 `PlanName`。
- `format.go` 的 `Check(..., ignoreMigration=false)` 在读标记前执行 `checkMigration`；活动计划缺少可信完成点时返回 `ErrMigration`；`RequirePath` 逐层检查标记和屏障。
- `internal/state/migration.go:268–273`：公共 `MigrateState` 调用 `migrateState(version, nil)`，故障注入只在内部测试接缝存在，没有受支持的生产 CLI 故障开关。
- `migration.go` 持主 Writer 和四个维护 Writer，验证实例/清单后发布计划，备份、切换、完成点与屏障清理均经固定流程。

现有 `n01-completion.../native.py` 先完成 `state-migrate --confirm`，确认活动计划已删除，随后测试旧程序的 `init/pubkey/serve/grant` 拒绝。它没有在源标记仍为 v1、活动计划已经发布的窗口调用旧程序，因此不能作为旧二进制屏障测试。旧轮 `native-format-drill.py` 仅覆盖未来/旧/损坏/重复/尾随/链接/未知目录与无标记读取，也没有活动屏障案例。旧轮报告提到拒绝现存 migration-plan，但没有对应完整源码快照可以在这里证明逐入口行为。

## 3. 下一批先取得的材料

1. 从维护方保留的旧审查快照取得完整、不可变的源码树或“准确基线 + 完整补丁”，以及构建命令、Go 版本、源清单、旧二进制摘要。只取得 Linux 二进制或文件摘要列表仍不足以构建 Windows 候选。
2. 对恢复的历史源码形成可追溯的 Git 对象身份，记录真正的 40 位源码 SHA 与原快照内容对照；保留“历史未发布的受支持 v1 防护快照”身份，不冒称当时已发布或已提交。不得把当前程序修改 Version、ReaderVersion/WriterVersion 或 marker 当历史源码。
3. 逐项证明旧程序 reader/writer 支持范围为 1；若实现只有四字段 format 标记，则需要核实完整读写边界和支持窗口，不能把格式号直接当成两个 reader/writer 声明。活动计划存在且 v1 标记不变时，旧程序的普通读/写入口必须先拒绝，且不能创建锁、密钥、配置、业务记录。
4. 若原旧快照不具备所需屏障，直接记录它不满足本批旧候选条件。需要新增兼容基线时另行设计、审阅、固定新身份，不能改动历史快照后继续声称它就是历史旧版。

以上满足后才能填写本方案缺失的 `old_source_sha`、Windows 旧 binary SHA、构建 provenance，并开始真实双二进制旅程。本轮不向维护方发送消息、不请求或运行远端归档。

## 4. 可复用测试及必须调整的地方

| 现有入口 | 下一批可复用部分 | 证据限制 |
| --- | --- | --- |
| `docs/evidence/personal-experience/n01-completion-20260913-190637/native.py` | 旧程序真实 init/pubkey/admit/grant/revoke；新程序显式迁移；历史数据与递归备份摘要；旧程序四入口拒绝；撤销保持与重复迁移幂等 | 固定 Linux 临时路径、arm64 二进制、locale text 解码、30 秒预算。需要新的 Windows 包装器显式传入已核实旧/新 binary、私有输出根和 UTF-8，保留原脚本；它本身没有旧版活动屏障或 NTFS ACL 验证。 |
| `internal/state/migration_test.go` | 完整备份与每个检查点/备份项恢复、漂移/计划篡改、锁忙、错误身份、缺失标记恢复 | `migrationFixture` 用当前代码初始化后写合成 v1 marker；只能计组件测试，不能计真实旧版数据迁移。 |
| `internal/stateformat/format_test.go` | `TestV2StrictContract` | 格式边界组件测试。 |
| `internal/statefs/fs_test.go` | `TestIndependentWritersAndReadersRejectChangedFormat`、`TestNestedMarkerCannotShadowOuterState` | 独立读写入口与祖先检查组件测试。 |
| `internal/clientrelease/state_compatibility_test.go` | `TestStateBoundReleaseBeforeAnyStaging` | 签名发行预检组件测试，不能代替实际双版本切换/回退。 |
| `scripts/personal-experience/windows-state-native-smoke.py` | 本批实际 Windows marker 负例、只读诊断、前后树快照 | 头部已明确不证明旧→新迁移；不能用此脚本合成 marker 替代旧二进制创建状态。 |

有可信旧候选后的 Windows 主旅程应使用同一真实目录、实例与历史对象：旧 binary 创建 v1 状态并产生可验证的已撤销 Grant；保存私有完整快照摘要；新 binary 显式迁移；逐项核验除兼容元数据/新迁移产物外的原始业务字节不变，备份覆盖完整原清单，撤销状态仍由新 binary 读回；旧 binary 再执行 init/pubkey/serve/grant，全部拒绝且全树不变；新 binary 的 state-status 与重复迁移保持只读/幂等。不要复制已绑定状态到新目录冒充同实例回退。

活动屏障与真实进程中断另列子旅程：通过迁移引擎实际产生计划，受控测试新进程中断，再测试旧/新普通入口拒绝与同命令恢复。当前没有生产故障开关；需要新增已审阅且单独标识的测试 harness 或可靠外部观察点，不能手写计划/删除 marker 作为“真实崩溃”。Windows 的陈旧 Writer 锁仍有保守拒绝限制；遇到锁拒绝必须保留失败，不手动删锁或关闭防护换取成功。每检查点可控故障注入、进程终止、真实断电耐久性必须分别记账。

## 5. 仅供下一批执行的命令模板

本节的构建和测试命令没有在本轮运行；下面的轻量 Git 查询已用于本轮只读核查。源码身份未确认前，不创建旧 worktree 或运行旧 binary。以下查询可重现本轮结论：

```powershell
git rev-parse --is-shallow-repository
git log --all --diff-filter=A --format='%H %s' -- apps/agentshield/internal/state/compatibility.go apps/agentshield/internal/stateformat/format.go
git show 9ba20e87c57b8ead40f4df3e090301bea77da6fa:apps/agentshield/internal/stateformat/format.go
git ls-tree -r --name-only ff99317450784c9563f5b6a2308c98e8262df98d -- apps/agentshield/internal/stateformat apps/agentshield/internal/state/compatibility.go
git grep -n -E 'migration-plan|state-format|ReaderVersion|WriterVersion|RequireStateCompatibility' ff99317450784c9563f5b6a2308c98e8262df98d -- apps/agentshield
```

下一批在各自干净、已核实的源码树执行标准构建命令，`<output>` 必须替换为新建私有测试根下的旧/新不同文件；不使用 `-X main.Version` 冒充旧身份：

```text
# cwd: each verified source tree / apps/agentshield
go build -trimpath -o <output-windows-amd64.exe> ./cmd/agentshield
go version -m <output-windows-amd64.exe>
```

记录真实 Go 版本、GOOS/GOARCH/CGO_ENABLED、源码前后 dirty 状态和二进制摘要；对缺少 VCS metadata 的快照构建必须明确标识来源，不伪填 `vcs.modified=false`。本机历史敏感输入要保持 Git blob 字节，不先批量格式化旧源码。

下一批相关组件子集（仅补充原生旅程，避免不必要全仓负载）：

```text
# cwd: new verified source tree / apps/agentshield
go test -p 1 -count=1 -timeout=10m ./internal/state -run 'Test(Migration|UnmarkedMigration|InitializeHistoricalState|BoundInitialization)'
go test -p 1 -count=1 -timeout=10m ./internal/stateformat ./internal/statefs
go test -p 1 -count=1 -timeout=10m ./internal/clientrelease -run '^TestStateBoundReleaseBeforeAnyStaging$'
go test -p 1 -count=1 -timeout=10m ./cmd/agentshield -run '^TestStateProtocolCLIContractFixtures$'
```

这些测试即便通过，仍不能补齐缺失的真实旧 v1 源码身份、旧程序活动屏障拒绝、Windows 双 binary 旅程、真实进程恢复或 NTFS ACL/断电证据。当前 N01 的 Linux 最低验收结论保持其原范围，Windows 迁移项继续未验证。
