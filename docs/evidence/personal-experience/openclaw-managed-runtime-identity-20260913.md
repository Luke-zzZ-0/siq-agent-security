# M125 证据：OpenClaw Runtime Identity 与 managed 接入（服务端/适配器安装层）

日期：2026-09-13　批次：M125（UX-004/UX-005 延伸至 OpenClaw 平台）
范围声明：本批完成的是**服务端与适配器安装层**的 OpenClaw 托管接入。OpenClaw 原生捕获与插件运行时桥接（M126）尚未实现，未经验收的组合一律不写 supported。

## 1. 平台模型

- `internal/runtimeidentity/store.go`：`supportedIdentityPlatforms = {"hermes","openclaw"}`（封闭集合，注释明确扩展需真实适配器与合同）。Runtime Identity Record 携带 `platform` 字段并整体进入签名；Summary 输出含 `platform`。
- Grant 平台锁定：OpenClaw 身份的签发要求对应 Grant 以 `grant.Options{Platform: "openclaw"}` 构建（`intent/grant_selection.go`）。创建时校验 Grant 平台与请求平台一致，凭据注册（enroll）时按 `AuthorizeSessionContext(token, platform, agent, session)` 一次读锁校验身份+绑定+平台，避免先认证再解析会话的窗口。
- 实例 ID 为内容派生（`hi-` = `hermeshome.Identifier(root)`），因此归属天然唯一：服务端解析 `resolveInstancePlatform` 依次尝试 Hermes、OpenClaw 根目录，命中即返回平台，不信任客户端上报。

## 2. 托管安装计划（adapterinstall）

- `validManagedTarget` 放行 Hermes 与 OpenClaw；`pinRuntimeIdentity` 校验身份元数据 `platform == o.Platform`（拒绝跨平台身份钉入）。
- 托管连接字段按宿主约定区分（`managedConfigKeys`）：
  - Hermes：snake_case `runtime_identity_id` / `token_path` / `agent_id`，写入 `<root>/plugins/siq-agent-security/config.json`；
  - OpenClaw：camelCase `runtimeIdentityId` / `tokenPath` / `agentId`，写入 `<root>/siq-agent-security.json`（product.Name）。
- `ConfiguredRuntimeIdentity` 按平台读取对应键与路径；`managedConnectionMatches` 同步按平台匹配，防止静默降级（删掉 ID 后重装被 `ErrPlanChanged` 拒绝）。
- 卸载继续要求先撤销身份（`ErrIdentityWithdrawalRequired`，tombstone `revoked:true` 后才放行），与 Hermes 保证一致。

## 3. 服务端 HTTP 面

- `/v1/adapter/instances?platform=openclaw`：OpenClaw 行（`openClawInstanceRow`），顶层 `native_available:false` —— 原生捕获未验证前不申报可用。
- `/v1/adapter/preview|install|uninstall`：OpenClaw 预览产出 `local-adapter-plan/v3`（含 runtime identity pin）；跨平台预览（对 OpenClaw 实例提交 hermes 平台）409。
- 安装后 `<home>/.openclaw/siq-agent-security.json` 含 `runtimeIdentityId` / `agentId` / `tokenPath`（= 服务端签发的凭据文件路径），插件资产 `plugins/siq-agent-security/index.ts` 落盘。
- 凭据会话按 openclaw 平台 enroll 后，裁决面 `platform=openclaw` 正常 allow/valid；用同一凭据以 hermes 平台裁决被拒 401（平台锁定负向）。
- 诊断 `diagnoseInstance` 对 OpenClaw 实例 `adapter_files` / `service_configuration` / `host_registration` / `instance_authority` 全部 pass。

### 修复的真实缺陷

`internal/server/adapter_http.go` `validateManagedSelection` 原先硬编码 `summary.Platform != adapterinstall.Hermes`，导致任何 OpenClaw 托管选择都被判 `ErrPlanChanged`（409「平台配置已变化」），managed OpenClaw 安装经 HTTP 完全不可用。已改为由计划视图透传平台：`s.validateManagedSelection(view.Platform, view.RuntimeIdentityID, view.InstanceID)`（preview 与 mutate 两个调用点同步）。

## 4. 测试与验证记录

新增/覆盖的定向测试（均 PASS）：

- `internal/runtimeidentity`：`TestOpenClawIdentityIssuedAndPlatformLocked`（签发、平台锁定、其余凭据规则不变）。
- `internal/adapterinstall`：`TestOpenClawManagedPlanWritesProductConfigFields`、`TestOpenClawManagedConfigurationCannotBeSilentlyDowngraded`、`TestOpenClawManagedPinRejectsForeignPlatformIdentity`、`TestOpenClawManagedUninstallRequiresIdentityWithdrawal`；既有 OpenClaw 诊断/注册/回滚回归不变。
- `internal/server`：`TestOpenClawManagedInstallDecisionsAndRevocation` 端到端（实例列表 → 跨平台拒绝 → 预览/安装 → 托管字段断言 → enroll → 平台内裁决 allow → 跨平台裁决 401 → 诊断 → 卸载预览 + 同计划重放两次 200 + 撤销后凭据失效）。

全量验证：

- `go test ./... -count=1` 全绿；`go vet ./...`、`gofmt -l`、`git diff --check` 无输出。
- 附带修复 `internal/skillinstall` 两处测试桩未跟随 `ResolveInstance` 签名扩展（`func(string) error` → `func(string) (string, error)`）导致的 vet 失败（removal_test.go / runtime_test.go）。
- `CGO_ENABLED=0` 四目标交叉编译通过，SHA256：
  - linux/amd64 `fa5baa3ea478e78d0f1ca1e72900b136cfaa7596d2a995e7c055bbaf365f2454`
  - linux/arm64 `f4d5ed0d032f9a5d87fd67d2447307c34bcdfc54d41e50e0dd97312202bb2542`
  - darwin/arm64 `95d883671d16fdfb4c03483028b44ee9fc021d534720218079714085bc8a4286`
  - windows/amd64 `7a885677ba968757ac65e13dea942d712be092dbf89d5e9d938887abf89ea2af`
- Python 合同：`uv run pytest app/tests/test_schema_contracts.py` 197 passed；`uv run ruff check app` 通过。

## 5. 未完成 / 边界（不得借本批提升支持）

- OpenClaw **原生捕获**未验证：`native_available` 持续报 false；runtime_check 自检仍为 Hermes 专属能力。
- OpenClaw 插件运行时桥（会话注册、裁决上下文、native-captures 提交）为 M126 待办。
- 真机验证仅覆盖本机 Linux arm64 宿主；macOS/Windows 实机、真实 OpenClaw 宿主进程为独立验证线。
- WorkBuddy 独立接入维持 blocked。
- 本批仅本地落盘，不提交、不推送。
