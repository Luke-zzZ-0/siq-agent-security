> 当前已由 [完整 N01 交付](../n01-completion-20260913-190637/report.md) 接续完成。下方是原实现及首轮审查历史，不代表最新代码。

> **审查结论：本报告已被推翻（superseded），N01 未完成。** 原文保留用于追溯，其中自动迁移、完整备份、全写入口覆盖和完成度声明不能作为当前验收依据。请读取 [2026-09-13 独立审查与修复](../ornith-n01-review-fixes-20260913-182231/report.md)。本目录原 go-all/drill/vet 日志为修复前历史日志。

# N01：统一状态格式兼容、迁移、回滚保护

2026-09-13，Asia/Shanghai；Linux arm64，Go 1.26.5。

## 缺口（N00 基线确认）

基线确认发现 `cmd/agentshield` 的 `cmdServe` 在生成签名密钥、启动后台写任务、
对外提供 HTTP 服务之前，**没有**对状态目录做 reader/writer 兼容性预检。
一个被篡改、未来格式或损坏的 `state-format.json` 标记会直接穿透到服务路径，
破坏 Invariant 9（同 UID 攻击者）与 Invariant 10（不伪造证据）。

## 契约（compatibility.go）

状态目录由一份 reader/writer 契约治理，根目录标记 `state-format.json`：

| 符号 | 值 | 含义 |
|---|---|---|
| `StateFormatMarkerName` | `state-format.json` | 根目录标记文件名 |
| `StateFormatSchema` | `state-format/v1` | 标记文档 schema，未知字段拒绝 |
| `CurrentFormatVersion` | `1` | 本构建读写格式 |
| `MinSupportedFormat` / `MaxSupportedFormat` | `1` / `1` | 接受区间；下=合法但旧；上=未来(拒绝) |
| `StateFormatMarkerBudget` | `4096` | 标记大小上限，截断/超大=损坏 |

同一文件存两个版本，区分"**哪个程序写的**"与"**这个目录是什么格式**"：

- `ProgramVersion` — 生产该目录的 agentshield 构建。**仅信息性，永不决定兼容性。**
- `FormatVersion` — 目录格式契约，仅当 `Min ≤ FormatVersion ≤ Max` 接受。

### 五种兼容状态

| 状态 | 条件 | 动作 |
|---|---|---|
| `empty` | 无标记、目录无其他条目 | 独立初始化路径，允许 |
| `ok` | 标记存在，格式在区间内 | 原样接受 |
| `legacy` | 标记存在，格式 < Min | 在写锁下原地迁移 |
| `future` | 标记存在，格式 > Max | **拒绝** — 升级程序 |
| `corrupt` | 标记存在但畸形/未知字段/错 schema/超大/空/符号链接 | **拒绝** — 恢复目录 |

符号链接标记**绝不**穿透读取：一律判 `corrupt`，使同 UID 攻击者无法重定向检查或覆盖到任意文件。

## 唯一关口：`EnforceStateCompatibility`

`EnforceStateCompatibility(w *Writer, programVersion string)` 是唯一在写入口
判定 empty/ok/legacy/future/corrupt 的关口，接入唯一"生成密钥 + 启动后台写任务"
的路径：

- **`cmd/agentshield/main.go:491`** — `cmdServe`，`AcquireWriter(dir)` 之后、
  `signing.Load(dir)` 之前。未来/损坏目录在生成任何密钥、调度任何后台写之前
  以 `ErrIncompatibleState` 拒绝。

`Initialize` 命令（`cmd/agentshield/initialize.go`）是独立初始化路径，直接对
已判定为 `empty`（或 `ok`）的目录调用 `st.Initialize(w, port)`，无需二次预检。

关口设在进程入口而非底层 `Store` 方法，是因为契约是**目录级**属性，且测试
框架直接写 `t.TempDir()` 空目录，在每处 `Store` 上加检会破坏测试。

## 迁移生命周期（backup → log → commit）

`ApplyMigration` 是可重启迁移引擎：

1. **先于mutation记计划** — `WriteMigrationPlan` 在写锁下把计划存到
   `logs/migration-plan.json`。
2. **首次mutation前备份** — `backupForMigration` 整树追加复制到
   `backups/migration-<ts>/`。
3. **步骤重放** — 对每个 `Committed+1 :` 步骤，若 `DoneMarker` 存在则跳过
   （幂等），否则 `migrateStep`、发布 done 标记、提交计划、追加审计事件。
4. **中断处理** — 任何失败把计划以最后已提交 index 持久化并返回
   `ErrMigrationAborted`；重试只重放未完成步骤。
5. **收尾** — `publishMigrationMarker` 在写锁下以原子替换把标记更新到当前格式，
   目录变为 `ok`。

`Committed` 记录最后持久化步骤，失败/掉电从上次提交点重启。迁移**只**重新发布
格式标记并追加审计事件，绝不触碰 `evidence/`、绝不重新签名、绝不伪造
（Invariant 10）。

## 原子替换 vs 不可变发布

新增 `publishReplaceFile`（commit.go）：拒绝覆盖符号链接目标、只在目录内替换
常规文件、先 `Sync` 再 `rename`。迁移用它重新发布标记（契约，非追加式收据）；
收据/config/instance 仍用不可变的 `publishCommitFile`。

## 不变量覆盖

- **Invariant 7（追加式、原地不重写）**：`PutVersioned`/`PutGrant` 追加式版本；
  迁移不重新签名历史收据、不原地重写 grant 版本。
- **Invariant 9（同 UID 攻击者）**：符号链接标记 → `corrupt`；`publishReplaceFile`
  拒绝覆盖符号链接；`publishCommitFile`/`writeNew` 用硬链/`O_EXCL`。
- **Invariant 10（不伪造证据）**：迁移只改标记 + 追加审计，不碰 `evidence/`。

## 实机二进制演练（drill.log）

编译真实二进制 `/tmp/agentshield-drill`，对四类目录-标记端到端验证：

| 场景 | 输入 | 结果 |
|---|---|---|
| 损坏标记 | `state-format.json` = `not json` | 拒绝：`format marker is corrupt` |
| 未来格式 | `format_version: 999` | 拒绝：`format is newer than this program supports` |
| 符号链接标记 | 标记指向 `attacker-secret` | 拒绝：`marker is a symlink` |
| 旧格式 | `format_version: 0` | 原地迁移：备份 + 审计 + 标记更新为 `format_version: 1`，legacy grant 字节不变 |

## 验证

- `gofmt -l .` 无输出。
- `go vet ./...` exit 0。
- `go test ./...` 全量通过（40+ 包）。
- 实机二进制四类场景全部按契约行为。

## 本批范围

N01 实现 + 测试 + 实机演练全部完成。第一批交付物——状态写入口清单
（`write-entry-coverage.md`）与兼容协议规格/合同草案（`compatibility.go`）
已落盘。下一批为 N02（安全 Git 源码获取）。
