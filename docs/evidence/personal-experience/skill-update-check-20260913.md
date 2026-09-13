# M132：UX-010 增量 —— 已安装 Skill 新版检查（只读远端快照）

日期：2026-09-13。任务书 UX-010（远端新版检查）。本地候选基于当前工作区，未提交或推送。

新版检查回答一个问题：上游是否出现了比已安装内容更新的版本。它是纯只读操作——不创建、不修改、不固定任何记录，不授予任何权限，不写入状态；唯一产物是一次性检查结果。若上游内容与已安装快照存在差异，结果置 `status=new_version` 且 `requires_confirmation=true`，将后续动作显式交还给既有更新比较/确认流程。权限差异不在本层比较（`permission_comparison=deferred_to_update_comparison`），因为候选授权尚不存在。

## skillimport 层（internal/skillimport/upstream.go）

- `CheckUpstream(ctx, importID, remoteURL)`：读取导入记录，暂存目录（`upstream-*`，0700，结束即 `RemoveAll`）内重新获取上游，产出 `UpstreamSnapshot{SourceKind, URL, Ref/SubDir/CommitSHA 或 ArchiveSHA256/ArchiveBytes, Directories, Files, ExcludedGitMetadata}`。
- git 来源：按记录中的 URL/Ref/SubDir 重新克隆（`ExpectedCommit` 为空即不固定），快照携带当前 HEAD `CommitSHA`——上游前进时快照如实反映新提交，记录保持原样。仅 https，复用 M131 硬编码 git CLI。
- https_zip 来源：调用方提供 URL，先以 `sum(canon.Marshal{url, archive_path, expected_sha256})` 复算定位摘要并与记录 `SourceLocatorDigest` 比对——URL 与记录绑定，改用其他 URL → `ErrChanged`（409），非 https → `ErrURLBlocked`。`ExpectedSHA256` 为空即不固定归档内容。
- local_dir / 未知 schema / 未知来源类型 → `ErrInvalid`（本地来源没有可检查的上游）。
- `ReadRecord` 导出包装：即使已安装侧清理了暂存副本，检查仍可读取导入记录。

## skillinstall 层（internal/skillinstall/update_check.go）

- `CheckUpdate(ctx, installID, req)`，合同 `local-skill-update-check/v1`，`UpdateCheckResult` schema `local-skill-update-check-result/v1`。前置校验：schema、RemoteURL ≤2048、ActorID 非空白。
- 前提条件：已安装记录 `RecordedStatus=installed_unverified` 且 Operation 存在（否则 `ErrChanged`）；无进行中的移除（`ErrRemovalPending`）；导入记录 `ArtifactDigest`/`AnalysisSHA256` 与 `Plan.Source` 绑定一致（篡改 → `ErrChanged`）。
- 来源分派：git 来源拒绝调用方 URL（上游位置以记录为准），https_zip 来源要求非空 URL 并交由 skillimport 层定位摘要绑定；本地来源 → `ErrInvalid`。
- 内容差异复用更新比较的共享 `compareContentDelta`（同一 200 项截断预算）：Total>0 → `new_version`；返回 `UpstreamCommitSHA`/`UpstreamArchiveSHA256` 供确认界面展示。
- 结果产出前重读记录并核对签名（检查期间被替换 → `ErrChanged`）、复查移除状态、检查 ctx 取消；边界事件 `update_checked`。检查全程不写入 skill-installations 目录（测试以路径清单前后对比断言）。

## HTTP

- `POST /v1/skill-installations/operations/{id}/update-check`：严格平铺 JSON（schema_version/remote_url/actor_id 必须作为键出现），管理员能力 401/403 边界，GET 405，槽位 TryLock 429，60 秒上下文；错误映射沿用 `skillInstallError`。

## 测试

- skillimport 3 项（`upstream_test.go`）：git 检查看到前进的 HEAD 与新文件、记录不动（CommitSHA/签名/ArtifactDigest 不变）、无 `upstream-*` 暂存残留；zip 定位摘要绑定调用方 URL（他 URL → ErrChanged，非 https → ErrURLBlocked）、换归档返回新 ArchiveSHA256、记录不动；local/未知 → ErrInvalid、不存在 → ErrNotFound。
- skillinstall 3 项（`update_check_test.go`）：up_to_date（同快照，断言 Status/RequiresConfirmation=false/PermissionComparison/CheckedAt/InstallID）与 new_version（差异文件 Before=nil/After 非空、新 CommitSHA、RequiresConfirmation=true），两次检查后 skill-installations 目录路径清单与权限 grant revision 均不变；210 项差异 → Total=210/返回 200/Truncated（200 截断预算）；守卫表（schema、空/空白 ActorID、未知 install id → ErrNotFound、local_dir → ErrInvalid、git 来源拒绝 RemoteURL[断言缝未被调用]、zip 来源拒绝空 RemoteURL[同]、导入 ArtifactDigest 篡改 → ErrChanged、revoke+Remove 后 → ErrRemovalPending 且缝未被调用）。git/zip 记录由测试以共享密钥重签落盘（canon.Decode → 变更 → 删签名 → SignCanonical → 0600 写回），保留 ArtifactDigest/AnalysisSHA256 维持安装绑定。
- server 1 项（`skill_update_check_test.go`）：401/403 凭据边界、GET 405、严格体循环（`{}`、null+重复键、额外键）全 400、本地来源 → 400 skill_install_invalid（含 git URL 传入同样 400）、未知 id → 404 skill_install_not_found、检查后移除状态仍 not_requested 且 grant 签名与 state_revision 不变。

## 验证

- `go vet ./... && go test ./...`：37 个含测试包全部 ok，0 失败。
- `gofmt -l` 与 `git diff --check` 无输出。
- 四目标构建通过（apps/agentshield，`./cmd/agentshield`，CGO_ENABLED=0）：

| 目标 | SHA-256 |
| --- | --- |
| linux/amd64 | fcf16558d0b16487ea5b417a17fc351533180bd52c1232e90eedb0ebd3a8e737 |
| linux/arm64 | 63fc9cea02b05501910d4a90abf04fc7702844ade908637434980e4e8a029efb |
| darwin/arm64 | 1b89903a1c2b177b7ae3207e10b308ff5b3e0de07ff960e15f29cae50d58bdd3 |
| windows/amd64 | fbd6367491a49995ff373a828fcbf237cc4d472b408d943239a6051e206bf525 |

- `uv run pytest app/tests/test_schema_contracts.py`：197 passed；`uv run ruff check app`：All checks passed。本轮未触碰合同 schema 与签发样本。

## 边界

- 检查中实际的重新获取（git 克隆/zip 下载）在 skillinstall 层测试经 `upstream` 缝以快照替身完成（跨包行为验证，无网络）；缝为边界测试专用，代码注释明确"Never set from runtime configuration"。真实获取路径由 skillimport 层测试以本地 git fixture 与直接 `download` 缝覆盖。
- 未在真实 git 托管服务或归档 CDN 联测；无 OS 实机验证。
- 权限差异、确认切换、原生更新验收与通用旧状态写入拒绝仍属 UX-010 后续，本轮未实现；UX-010 保持 doing。
