# 证据：Skill 运行时归属与权限绑定（M128 / UX-007 核心批）

日期：2026-09-13
范围：apps/agentshield（agentshield 引擎层）+ packages/contracts（schema）
对应任务书：UX-007 智能体与 Skill 权限编辑及运行绑定；Q05 Skill 身份可信性
性质：仅本地落盘，未提交、未推送、未发布。

## 1. 设计

### 1.1 威胁模型（UX-007 验收映射）

- 同一 Skill 在两个智能体中的权限可不同 → 权限按 grant 归属（platform+agent+skill 版本身份）签发，归属裁决按 platform+agent 精确匹配。
- 模型伪造 Skill 标识 → 运行时 `receipt.Request.Skill` 是**不可信声明**；`verified` 只能由引擎对受信本地状态（签发的 grant）精确匹配得出。
- 切换版本 / 借用其他 Skill 权限 → content_hash、version、skill_id、platform、agent 任一不匹配即 `mismatch` / `unknown`，skill 范围的 grant 拒绝行使。
- 未知归属不显示为已验证 → 状态只有 `verified|mismatch|unknown`；`unknown`（含声明格式损坏）按未知呈现，损坏的身份字段不回显进签名的 receipt。

### 1.2 数据流

1. 授予侧：admission.Admit 从 SKILL.md 内容推导 `SkillID`（`source:type:name@hash12`）、`SkillVersion`、`ContentHash`；grant.Build 将其固化为 `Grant.Skill`（`skill_id` + `content_hash` + 可选 `version`），纳入签名 canon。
2. 运行侧：适配器在 `receipt.Request.Skill` 携带运行时声明（不可信）；引擎经 `state.Store.SkillAttribution`（可插拔 `SkillAttributionLookup`）对受信 grant 记录裁决，结果 `SkillAttribution`（status + 受信身份）签入 receipt。
3. 决策侧：skill 范围的 grant 仅在归属 `verified` 时可行使；伪造/换版/借用/未声明 → 默认拒绝，reason_code `skill_attribution_mismatch`。

### 1.3 分阶段强制（关键决策）

`SkillAttributionEnforced` 选项 / `skill_attribution_enforcement` 配置项（默认关）。原因：平台适配器尚未附加运行时 Skill 声明，立即无条件强制会拒绝所有真实 skill 派生 grant 的每次运行时调用（实测使 runtimecheck 探针与 server 流程全部拒绝）。强制关闭期间：

- 声明仍被解析、裁决并签入 receipt（诚实遥测）；
- skill 范围 grant 暂按基线行为行使；
- 任何调用都不会因声明而显示 `verified`，除非 lookup 确认。

`native_available` 等运行时探测不受本批影响。

## 2. 改动清单

- `internal/grant/skill_ref.go` + `skill_ref_test.go`：Skill 版本身份进 grant 与签名。
- `internal/state/state.go`：`SkillAttribution` lookup + `Config.SkillAttributionEnforcement`（JSON `skill_attribution_enforcement`, omitempty）。
- `internal/receipt/engine.go`：声明解析 → lookup 裁决 → `rec.SkillAttribution` 签入；`skillAttributionMatches` 门禁（gated on `SkillAttributionEnforced`）。
- `internal/receipt/skill_attribution_test.go`：10 个测试。
- `internal/state/skill_attribution_test.go`：6 个测试。
- `cmd/agentshield/main.go`：配置接线。
- `packages/contracts/receipt.schema.json`：新增 `skill_attribution`（status 必填；verified 时必须携带 skill_id+content_hash；`additionalProperties: false`）。
- `packages/contracts/grant.schema.json`：新增 `skill`（skill_id+content_hash 必填，version 可选）。
- `cmd/agentshield/main.go`：`cfg.SkillAttributionEnforcement` → `SkillAttributionEnforced`。

## 3. 合同样本再生（testdata/contracts）

grant canon 新增 `skill` 字段导致签名/派生 ID 变化，按机制用 `AGENTSHIELD_UPDATE_SAMPLES=1` 再生 6 个样本；逐个人工核对 diff 仅为预期变化：

- grant-draft-created.json：+skill 块（local_dir:github-triage@4af17a9f9719, v1.2.0）。
- local-skill-import-permission-created / local-skill-install-runtime-readiness / local-skill-update-stage-create / local-skill-update-transaction：+skill 块 + 相应签名变化。
- local-skill-update-plan：update_id/install_id/plan_id/签名连锁变化（canon 变化所致）。

## 4. 测试证据（16 个新增测试，覆盖正负向）

receipt 层（强制开启下）：verified 放行；unclaimed 拒绝且 receipt 无 skill_attribution 字段；伪造未知身份拒绝；版本切换拒绝；跨智能体借用拒绝且记 unknown；基线 grant 不受声明影响；5 种畸形声明保持 unknown 且不回显；nil lookup 全 unknown；lookup 返回非法状态值按拒绝处理；生命周期链上归属随决策追加。

state 层：精确匹配 verified；内容漂移/换版/缺 hash/缺 version → mismatch；无关 Skill → unknown；跨智能体/跨平台 → unknown；过期/非 live grant → unknown；基线 grant 永不 verified。

命令与结果（apps/agentshield）：

- `go test ./...` → 36 packages ok，0 failures（含 16 个新增）。
- `gofmt -l .` → 无输出；`git diff --check` → 无输出；`go vet ./...` → 通过。

## 5. Python 合同测试（apps/control-api）

- `uv run pytest app/tests/test_schema_contracts.py -q` → 197 passed（schema 改动后全部通过）。
- `uv run ruff check app` → All checks passed。

## 6. 四目标构建（CGO_ENABLED=0，./cmd/agentshield）

| 目标 | SHA-256 |
|---|---|
| linux/amd64 | 69983d8b36bf798001537aeba95e8f9f0d56a2ee82f4cf1483ef35d12234c654 |
| linux/arm64 | ac5fad7738263ce79f09c8e28a877966c29d8baebdf9601c48d63df4bdfdc304 |
| darwin/arm64 | 11d6933c4033e882445956e04a00341c90f0a257685f3dc7bf46513e4c4f170c |
| windows/amd64 | f3ad863434571f94266d5abb71a84b224a659f569b743abf051e701f3a2c9f1b |

（windows/amd64 产物经 `file` 确认 PE32+ console executable x86-64。交叉编译/CI 工具测试不构成实机平台证据；未验收组合保持 blocked。）

## 7. 边界与遗留

- 强制默认关闭：`skill_attribution_enforcement` 打开前，skill 范围 grant 按基线行使；声明只进遥测。真正的端到端强制依赖**适配器侧运行时 Skill 声明附加**（后续批次的前置工作，已记入台账）。
- UX-007 整体仍为 doing：场景模板、多 Skill 边界、跨 OS 行为尚未覆盖。
- 原文采集、审计链、Secret 排除约束未触碰，无回归。
