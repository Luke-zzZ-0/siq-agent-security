# M127：OpenClaw 托管接入真机原生冒烟

> 本文保留 M127 原始测试事实；原 JSON 已归档为 [M127 原始报告](openclaw-managed-native-smoke-m127-original-20260913.json)。安装配置问题随后在产品侧修复并移除夹具 workaround，最新验证见 [阶段修复记录](stage-review-fixes-20260913.md) 和 [修复后原生报告](openclaw-managed-native-smoke-reviewed-20260913.json)。原生安装入口拦截仍未获得证明。

日期：2026-09-13。规格：个人体验任务书 OpenClaw 托管原生链路（native preview）；脚本 `scripts/personal-experience/openclaw-managed-native-smoke.py`；报告存档 `/tmp/openclaw-managed-native-smoke-final.json`（schema `personal-openclaw-managed-native-smoke/v1`）。

宿主：真实 OpenClaw 2026.5.12（`~/.nvm/versions/node/v22.22.1/lib/node_modules/openclaw`），公共 CLI 入口 `openclaw agent --local`，真实插件 hook 生命周期；合成模型与工具夹具，隔离 HOME，未触碰用户 daemon。daemon 二进制 SHA-256 `1d358cd5e411d158…`（linux/arm64，本仓构建）。

## 结果

`passed: true`，17 项检查全部通过：

openclaw_catalog_reports_native_unavailable（原生宿主未验收前 `native_available` 报 false）、managed_preview_does_not_mutate_host、managed_install_uses_issued_identity、installed_assets_match_adapter_source、installer_preserves_other_openclaw_settings_and_install_policy、openclaw_2026_5_12_rejects_security_installpolicy_key_moved_aside_for_run、diagnosis_keeps_runtime_unverified、native_cli_started、native_session_automatically_enrolled、instance_credential_used_without_manual_intent、environment_cannot_override_managed_agent、write_denied_before_execution、allowed_read、native_raw_parameters_captured_after_explicit_task_grant、native_raw_output_captured_after_explicit_task_grant、receipt_chain_verified（回执 3 条）、revoked_identity_blocks_new_native_calls。

raw_content：record_count 2（output、parameters 各 1），绑定来自服务端签名的原生会话绑定，grant `rawgrant-09b5840fb98b3a942b3286f53926bc0f`，明文经 admin 读取核对一致。安装走 managed_v3_preview_apply + 公共 OpenClaw CLI，变更文件 5 个，runtime_state 保持 unverified。

## 真机发现与处理

1. **安装器兼容性发现（仅夹具规避，产品未改）**：installer 在 openclaw.json 顶层写 `security.installPolicy`；OpenClaw 2026.5.12 严格校验将其判为 Unrecognized key 并拒绝启动 agent run。夹具将键移出后再启动，报告记入 `compat_finding`；后续应在 installer 侧改为 OpenClaw 认可的存放位置或配置子树。
2. **ctx.agentId 固定错误（产品缺陷，已修复）**：插件此前把宿主 agent id（`ctx.agentId`）当 `agent_id` 上报，managed 绑定固定在 `cfg.agentId`，导致 decide 401 `scoped_decision_credential_required`、全部 failClosed。修复：新增 `reportedAgentId(ctx)`——managed 路径恒用 `cfg.agentId`，非托管才回退 `ctx?.agentId ?? cfg.agentId`；decide 与 observe 均改用该值。
3. **undefined 叶子中断捕获（产品缺陷，已修复）**：OpenClaw 的 after_tool_call result 载体自带值为 undefined 的可枚举键（`details`、`terminate`）。`JSON.stringify` 会省略它们，但 `Object.entries` 会枚举到，undefined 叶子触发 `JSON.stringify(undefined) === undefined → return false`，静默中止整个 output 捕获（捕获本身 best-effort）。修复：`rawContentFields` 遍历对 `undefined`/`function`/`symbol` 叶子跳过而非中止；新增回归场景 `managed-output-capture-skips-undefined-leaves`（含 details/terminate 的载体仍完整产出 `/tool/result/content/0/text` 字段且不含这两个键）。
4. **tool-call id 改写（宿主行为，夹具适配）**：OpenClaw 2026.5.12 回放转录时剥除模型签发 tool-call id 中的非字母数字字符（`write-denied`→`writedenied`），同一会话内重复 id 追加 8 位 hex 消歧后缀（`writedeniedf7600b48`）。夹具按精确/剥除/剥除+8hex 三态配对。
5. **全量转录回放（宿主行为，记录）**：OpenClaw 每次请求回放完整会话转录，role=tool 消息数跨轮次累计；断言按轮次尾部切片。

## 资产双拷贝同步

插件规范源在 `adapters/runtime/openclaw-agentshield/index.ts`（插件测试与冒烟一致性断言用）；daemon 经 `apps/agentshield/internal/adapterinstall/assets/openclaw/index.ts` go:embed 内嵌副本安装。`TestEmbeddedAssetsMatchRuntimeTree` 强制两份一致；每次插件改动后必须 `cp` 重新同步，否则安装器校验 "installed asset differs from adapter source" 失败。

## 验证

- 插件测试套件 10/10 通过（含新增 undefined-leaf 回归场景与 managed-ignores-host-agent-id 断言修正：enroll 请求体仅含 schema_version+session_id，不参与 agent pin 断言）。
- `go test ./internal/adapterinstall/` 通过（资产一致性）。
- gofmt 无输出；`git diff --check` 干净。
- CGO_ENABLED=0 四目标构建（构建目录 apps/agentshield，目标 `./cmd/agentshield`）：

| 构建 | SHA-256 |
| --- | --- |
| linux/amd64 | 0a1fdef305f5f44bd9b145f8af53ea3f9156abd835af19ab56ec71298a3b29d8 |
| linux/arm64 | 124f4fc103d095e42f60dadcd641b85a26b5488a9206a607fa2e92474b104fc1 |
| darwin/arm64 | 4f7abab4000fcc22b3433a1a6f1689340a65307d34daab107668d1573737f76c |
| windows/amd64 | d8aad4e668077c59e2d4c79657db01b4e5cbd1ff7c3c5d9917aa4d1bfc189fa0 |

- Python 侧：`uv run pytest app/tests/test_schema_contracts.py -q` 197 项全过（[36%]/[73%]/[100%] 三行全点无失败标记）；`uv run ruff check app` "All checks passed!"（apps/control-api）。

## 边界

未宣称：真实用户审批、Skill 归属、OS 隔离、网络隔离、浏览器工作流；安装后 runtime_state 保持 unverified；`native_available` 仍报 false，直至 `security.installPolicy` 兼容性在产品侧解决并复跑真机验收。改动仅本地落盘，未提交、未推送、未发布，未重启用户 daemon。
