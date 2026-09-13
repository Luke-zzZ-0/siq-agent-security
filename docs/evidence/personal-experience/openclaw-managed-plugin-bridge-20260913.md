# M126：OpenClaw 插件运行时托管桥（managed Runtime Identity + 原生原文捕获）

日期：2026-09-13 ｜ 批次：M126 ｜ 范围：`adapters/runtime/openclaw-agentshield`（v0.2.0 → v0.3.0）及内嵌资产副本

## 1. 目标与范围

M125 在服务端/安装层为 OpenClaw 打通了 Runtime Identity 与托管安装计划；本批把托管运行时行为落到插件本体，对齐 M123 建立的 Hermes 托管桥合同：

1. 托管凭据校验（格式/符号链接/512 字符上限，旧全局 token 不得充当托管身份）；
2. 每次 `before_tool_call` 先经 `/v1/runtime-sessions` 注册会话并严格校验 8 字段响应；
3. allow 后捕获参数、observe 带决策引用后捕获结果，走 `/v1/raw-task-content/native-captures`（250ms best effort 预算，期望 201）。

非托管（legacy）路径行为不变：不注册、不捕获。

## 2. 实现（`index.ts`，与 `apps/agentshield/internal/adapterinstall/assets/openclaw/index.ts` 字节一致）

- 配置新增 camelCase `runtimeIdentityId`；`managed()` = `runtimeIdentityId` 匹配 `ri-[32hex]` 或 `agentId` 匹配 `hri-[32hex]`。
- `readToken()` 托管分支：`lstatSync` 拒绝符号链接、512 字符上限、必须匹配 `ri-<32hex>.<64hex>` 且前缀等于 `runtimeIdentityId`，否则置空（"A legacy global token must not silently act as the managed identity."）。
- `post()` 增加 expected-status 参数（捕获期望 201，其余 200），非期望状态返回 null。
- `enrollRuntimeSession()`：非托管直接通过；托管校验身份/agent/session（≤256 字符）后 POST `local-runtime-session-enroll/v1`，响应必须是且仅是 8 字段：`schema_version=local-runtime-session-enrolled/v1`、`identity_id`、`platform=openclaw`、`agent_id`、`session_id` 逐项一致，`binding_id=bind-[64hex]`、`intent_id=int-ri-[64hex]`、`expires_at` 非空。任何不满足 → false → block 模式 fail-closed（且不会到达 `/v1/decide`）。
- `rawContentFields()`：`/tool/name` + 根指针（参数 `/tool/arguments`、结果 `/tool/result`）递归展平为 JSON pointer 字段；深度 >32 / 路径 >256 / 路径含控制字符 / 字段 >1024 / 单值 >1MiB / 非有限数 / `undefined` 任一出现即整体放弃；`~0`/`~1` 转义；空对象 `"{}"`、空数组 `"[]"`。
- `captureNativeRawContent(kind, ...)`：守卫身份/agent/session 后 POST native-captures（`local-raw-task-content-native-capture/v1`，platform/agent_id/session_id/kind/fields），预算 250ms，失败/超时静默（best effort）。
- hook 接线：`before_tool_call` 在 decide 前注册；`allow` 后捕获参数；`after_tool_call` 先算决策引用（`action_id`/`decision_receipt_id`）随 observe 提交，仅在确有引用时捕获结果。

daemon 拥有原文采集策略与 secret 过滤；适配器不读原文开关，只提交原生字段。

## 3. 测试方法与结果

新增 `tests/`（不进入 go:embed 内嵌资产）：`resolve-hook.mjs`（module resolution hook 把 `openclaw/plugin-sdk/plugin-entry` 重定向到本地 stub）、`plugin-entry-stub.mjs`（`globalThis.__pluginEntry` 记录 spec）、`scenarios.mjs`（场景本体）、`managed-bridge.test.mjs`（`node:test` 入口，逐场景子进程隔离——插件在 import 时一次性读取配置与凭据）。

每场景以 mock 本地 HTTP 服务（127.0.0.1 随机端口，记录全部请求）+ 临时 `OPENCLAW_STATE_DIR` 驱动真实 hook handler，延续 2026-09-05 证据目录的"插件会发出的请求"L2 约定。8 个场景全部通过（`node --experimental-strip-types --test tests/managed-bridge.test.mjs`，Node v22.22.2）：

| # | 场景 | 断言要点 |
| --- | --- | --- |
| 1 | legacy-no-enrollment-no-capture | 请求序列仅 `/v1/decide`+`/v1/observe`；无注册/捕获 |
| 2 | managed-allow-enrolls-and-captures-params | 注册→decide→捕获顺序；enroll body `local-runtime-session-enroll/v1`；decide `platform=openclaw`/agent/session；捕获 kind=parameters、`/tool/name` 与 `/tool/arguments/path` 字段、Bearer 托管凭据 |
| 3 | managed-enroll-failure-fails-closed | 注册 500 → `{block:true}`（fail-closed），decide 未被调用 |
| 4 | managed-foreign-platform-enrollment-rejected | 响应 platform=hermes → 视同畸形，fail-closed |
| 5 | managed-legacy-token-rejected | 托管配置+旧 token → 零请求、fail-closed |
| 6 | managed-observe-with-reference-captures-output | observe 带 `action_id`/`decision_receipt_id`；第二条捕获 kind=output、`/tool/result/report` 字段 |
| 7 | managed-capture-timeout-is-best-effort | 捕获 1500ms 慢响应不阻断 allow 路径（250ms 预算内中止） |
| 8 | managed-deny-blocks-with-receipt | 托管 deny 照常阻断且带回执 `rcp-2` |

## 4. 全量验证记录

- Go（`apps/agentshield`）：`go test ./...` 36 个包全部 ok（含 `TestEmbeddedAssetsMatchRuntimeTree` 内嵌资产一致性）；`go vet ./...`、`gofmt -l` 无输出。
- `git diff --check`：无输出。
- CGO_ENABLED=0 四目标构建（`./cmd/agentshield`）SHA256：
  - linux/amd64 `f5b79118a7e5aed7af4c655628db60135f1864ea851972658aebfdd199485104`
  - linux/arm64 `6e3481839d6a914ac25744a9032df5dc6d4df896571ab2553483a7637ebde4b9`
  - darwin/arm64 `21fae26142bd1df9c33efd5e393c6e04f672b2a13c2743c3aceb1dbdc68173b5`
  - windows/amd64 `14bf83f8d72084cbf9f5100010c4ac898c1c14deafde129c982ccd2595dc4ef5`
- Python（`apps/control-api`）：`uv run pytest app/tests/test_schema_contracts.py` 197 passed；`uv run ruff check app` All checks passed。

## 5. 边界与未完成

1. **测试不是真实 OpenClaw 网关验收**：SDK 入口经 resolution hook 替换，事件由测试直接驱动；真实 `agent --local` 会话、网关审批、消息渠道不在本批覆盖。原支持矩阵不因此标注 `supported`。
2. **`native_available` 保持 false**：`/v1/adapter/instances` 顶层申报只有在真实原生捕获于实机 OpenClaw 验证后才可置 true（与 M123→M124 的 Hermes 路径同理）。
3. 原文默认不采集：适配器仅在 daemon 授权路径内"提交"字段，是否落盘由 daemon 原文策略决定；Secret/凭据仍由服务端 secret 过滤处理。
4. 仅覆盖本机 Linux arm64 宿主（交叉编译产物未实机运行）；WorkBuddy 独立接入仍 blocked。
5. 本批改动仅本地落盘，不提交、不推送、不发布。
