# M129 证据：场景模板与多 Skill 调用边界（UX-007 后半）

日期：2026-09-13
范围：apps/agentshield（Go 后端）、packages/contracts/grant.schema.json
状态：仅本地落盘；未提交、未推送、未发布；未重启用户 daemon。

## 1. 目标

完成 UX-007 剩余两项后端能力：

1. **场景模板（scenario templates）**：为非专家用户提供封闭目录的"收缩型预设"，签发 grant 时一次性应用，只能移除声明事实、永不新增。
2. **多 Skill 调用边界**：同一 agent 存在多个 live Skill grant 时，运行时归属必须按 Skill 身份精确匹配，杜绝跨 Skill 借用授权。

## 2. 设计要点

### 2.1 场景模板是纯收缩预设

- 封闭目录（`internal/grant/scenario.go`），共 3 个，全部为只减不加：
  - `no-network@1`：移除全部 network 声明（http.request / socket.connect）。
  - `no-exec@1`：移除 process 执行与 resource 包安装声明。
  - `sandboxed@1`：移除 network/process/resource 与 fs.write；仅保留工具、模型声明与隐式只读。
- `Build` 在派生任何投影（hermes allowlist、OpenClaw 工具策略、网络/文件系统规则等）**之前**用 `ApplyScenario` 过滤 `DeclaredFacts`，因此所有投影天然一致，不存在"模板只改了某一种投影"的旁路。
- 模板身份（`ScenarioRef{ID, Version}`）写入签出的 Grant 并参与签名；运行时/审计可证明"这份权限是哪个预设收敛出来的"。
- 失败关闭：`Build` 校验场景必须存在于封闭目录且版本精确匹配（id+version），否则 `ErrScenarioInvalid`；不支持"传入任意 Scenario 对象"绕过目录。
- 默认效果始终为 deny，模板不改变这一点。

### 2.2 向后兼容

- `Grant.Scenario` 为 `omitempty`：旧签名 grant（无 scenario 字段）验签与合约样本均不受影响——本批合约样本实际零变更，`grant.schema.json` 新增的 `scenario` 为可选属性。
- `DraftFrom` 克隆已签 grant（含 JSON 克隆 Scenario），从既有 grant 派生新 grant 时模板身份自动保留。

### 2.3 多 Skill 边界

- lookup 层（`internal/state`）：Skill 归属按 (platform, agent) 下的**每个 live Skill grant** 独立精确匹配 skill_id+content_hash+版本；两个 live Skill grant 互不干扰，混合身份（A 的 id + B 的 hash）必须 mismatch。
- 引擎层（`internal/receipt`，enforcement on）：同一 agent 有两个 Skill-scoped grant 时，live grant A 只服务归属为 A 的调用；声称 B 的调用即使其归属对 B 为 verified，也不得经由 A（或跨身份）获得授权，拒绝码 `skill_attribution_mismatch`。

### 2.4 适配器侧 Skill 身份声明挂载：阻塞于宿主能力（诚实记录，未伪造）

M128 将运行时归属设计为"引擎只信可信 lookup、claim 不被信任"，本批从两个真实宿主的源码确认了"适配器无法自行提供可信 Skill 身份"是宿主能力缺口而非实现遗留：

- **OpenClaw 2026.5.12**：`before_tool_call` 钩子上下文字段仅 `{toolName, agentId, sessionKey, sessionId, runId, toolCallId}`（来源：`/home/maoyd/.openclaw/node_modules/openclaw/dist/reply-BCcP6j4h.js` 约 33932 行）。
- **Hermes**：`pre_tool_call` 中间件仅收到 `(tool_name, args, task_id, session_id, tool_call_id, turn_id, api_request_id, middleware_trace)`（来源：`/home/maoyd/.hermes/hermes-agent/hermes_cli/plugins.py:6831`）。

两个宿主的工具调用钩子均不暴露"本次调用来自哪个 Skill"的运行时身份。因此适配器在钩子处无法给出可验证的 Skill 声明，M128 的 claim 挂载只能保持阻塞、enforcement 默认关闭。该结论随宿主版本演进需复核（若上游钩子加入 skill 来源字段，可解除阻塞）。

## 3. 变更清单

- `internal/grant/scenario.go`（新增）：封闭模板目录、`ScenarioRef`、`ApplyScenario`（只过滤 DeclaredFacts）、`ResolveScenario`/`ScenarioByID`/`Scenarios`、`ErrScenarioInvalid`。
- `internal/grant/grant.go`：`Grant.Scenario *ScenarioRef`（omitempty）与 `Options.Scenario`；`Build` 校验场景在封闭目录内后应用；`buildGrant` 把 `{id, version}` 签入 Grant。
- `internal/server/server.go`：`GET /v1/grant-scenarios`（只读、无敏感字段）；`POST /v1/grants` 接受 `scenario_id`，未知场景 400。
- `internal/state/skill_attribution_test.go`：新增多 Skill lookup 边界测试。
- `internal/receipt/skill_attribution_test.go`：新增引擎层多 Skill 边界测试。
- `packages/contracts/grant.schema.json`：新增可选 `scenario` 属性（文档化签名字段；样本零变更）。

## 4. 测试（正负向）

### 4.1 Go 单测

场景模板（`internal/grant/scenario_test.go`，7 个）：

1. `TestScenarioNoNetworkDropsNetworkFacts`：no-network 移除网络声明、保留工具/模型；场景身份签入且 Verify 通过；hermes allowlist 同步收缩。
2. `TestScenarioNoExecDropsProcessAndResource`：no-exec 移除进程/包安装（OpenClaw exec 门随之消失），网络保留。
3. `TestScenarioSandboxedIsReadOnlyShape`：sandboxed 后仅剩工具/模型声明 + 隐式只读。
4. `TestScenarioNeverWidensBeyondAdmission`：3 个模板结果 ⊆ 基线，且不出现基线没有的 (domain, action) 组合。
5. `TestUnknownScenarioRejected`：未知 id 查不到；`ResolveScenario` 报错；`Build` 拒绝目录外构造的 Scenario 对象（失败关闭）。
6. `TestScenarioCatalogIsClosedAndFailClosed`：目录恰为 3 项、身份字段齐全、默认效果 deny。
7. `TestDraftFromPreservesScenario`：sandboxed → 人签核 → DraftFrom，场景身份保留且验签通过。

HTTP 面（`internal/server/grant_scenarios_test.go`，2 个）：

8. `TestGrantScenarioCatalogEndpoint`：返回 3 项且无任何 secret/token/key/signature 字段；POST → 405。
9. `TestGrantCreateWithScenarioRestrictsAndBinds`：未知 scenario_id → 400；sandboxed → 场景绑定 {sandboxed, 1}、无 network/process/resource/fs.write 事实、默认 deny；基线签发响应无 scenario 字段。

多 Skill 边界：

10. `TestSkillAttributionExactAmongMultipleSkills`（`internal/state`）：两个 live Skill grant 各自精确验证；A 的 id + B 的 hash → mismatch。
11. `TestMultiSkillBoundaryOnlyMatchingGrantServes`（`internal/receipt`，enforcement on）：claim B 在 live grant A 下 → deny（`skill_attribution_mismatch`）；claim A → allow 且 receipt 记录 A 的 verified 归属与 matched grant。

### 4.2 验证组合

| 项 | 结果 |
| --- | --- |
| `go test ./...`（apps/agentshield） | 全部 ok（0 失败） |
| `gofmt -l .` | 无输出 |
| `git diff --check` | 无输出 |
| `go vet ./...` | 无输出 |
| 合约样本 | 零变更（`scenario` 为可选字段；未触发 `AGENTSHIELD_UPDATE_SAMPLES` 差异） |
| `uv run pytest app/tests/test_schema_contracts.py -q` | **197 passed**（与 M128 持平，确认样本/合约未破坏） |
| `uv run ruff check app` | All checks passed |

### 4.3 四目标构建（CGO_ENABLED=0，`./cmd/agentshield`，apps/agentshield 目录）

| 目标 | SHA-256 |
| --- | --- |
| linux/amd64 | 50a0fc47c377d5899a7d2df9e0e0b2b06db4b41e6f2afc808095479e4191119e |
| linux/arm64 | 20d876cb13e456fd9a4cad49581751af06eef9762fcb5eef58dec4f8df03e13d |
| darwin/arm64 | e616262a9e4aec22b499f11a1c3621918a125c30a2df50e1a68d961c306068ed |
| windows/amd64 | f0f07d6a27af0c5ba9b4c9fe69a09294f76c0c489ff0b4a3c33b47d45e776941 |

## 5. 边界与未竟事项

- **UX-007 仍为 doing**：场景模板与多 Skill 边界本批闭环；**跨 OS 行为**与**适配器侧运行时 Skill claim 挂载**未闭环。后者如 §2.4 所述阻塞于宿主钩子能力（OpenClaw/Hermes 均不暴露 Skill 身份），保持诚实阻塞，不伪造 supported/native_available。
- 场景目录为封闭集合；新增模板属代码变更（需测试与签发路径回归），HTTP 面不接受任意自定义模板。
- 多 Skill 边界在 state lookup 与引擎两层均有测试钉死；但生产部署下"同一 agent 多个 live Skill grant 的并存策略"（是否允许并存）属运营策略，后续批次（UX-009/010 安装与更新移除）处理。
- 仅本地落盘，未提交、未推送、未发布；未重启用户 daemon。
