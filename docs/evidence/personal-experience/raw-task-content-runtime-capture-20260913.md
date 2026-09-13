# M117 运行时会话绑定的原文采集许可与 API

日期：2026-09-13；任务：UX-013；规格：§3.12.26、ADR-048。

本批增加不落盘的短时签名 `local-raw-task-content-capture-permit/v1`。许可最长 300 秒，以随机 ID 标识，绑定原文 Grant ID 和完整签名，并只保存 Runtime Identity、原生会话、签名 Binding 及服务端任务的 SHA-256 引用和一个内容种类。到期点取请求期限、原文 Grant 到期和运行时 Binding 到期的最早值；许可不续期、不改变原权限，也不能独立作为 bearer credential 使用。

本地运行时服务新增：

- `POST /v1/raw-task-content/capture-permits`：要求已发行实例凭据和已绑定的真实运行时会话；核对平台、智能体、会话、服务端任务、active Grant、完整 Grant 签名和内容种类后返回 201 与短时许可。
- `POST /v1/raw-task-content/captures`：要求同一实例凭据，再次验证当前会话 Binding、Activation/独立密钥、许可签名与期限、原文 Grant 和撤销状态，然后写入独立 AES-256-GCM 仓并返回 201 与记录元数据。

管理会话、全局决策凭据、自检凭据、未登记会话和跨实例/会话/Binding/任务请求均不能使用这些入口。Runtime Identity、会话/Intent/权限 Grant 或原文 Grant 后续撤销/到期会阻止下一次采集。许可签名被修改、原文仓损坏、错误 Grant 签名、错误内容种类和磁盘预算不足均失败关闭。

采集正文按最多 2 MiB 请求读取，结构化字段仍受 1 MiB 规范明文和 Grant 单条上限约束。请求递归拒绝重复 JSON 键，permit 与每个字段只接受精确字段名，null/未知/大小写别名均拒绝。`secret=true`、敏感路径和内置凭据值模式在加密前整项删除；全部字段被删除则不创建记录。成功响应不含原文、nonce 或 ciphertext，磁盘封套中也没有可读原文。

验证通过：

- `go test ./...` 与 `go vet ./...`（`apps/agentshield`）。
- `go test -race ./internal/rawcontent ./internal/runtimeidentity ./internal/server -run 'Test(CapturePermit|RawTaskContentRuntime|SessionEnrollment)' -count=1`。
- Capture Permit、Permit Create、Capture 和 Capture Result 固定 Go 样例通过 Python Draft 7 合同；固定种子 Permit 由 Python 独立 Ed25519 验签。合同测试共 188 项，Ruff 通过。
- `CGO_ENABLED=0` 下 `linux/amd64`、`linux/arm64`、`darwin/arm64`、`windows/amd64` 四目标编译通过。
- `gofmt -l` 与 `git diff --check` 无输出。

边界：本批只提供本地服务协议，尚未把采集调用接入 Hermes、OpenClaw 或 WorkBuddy 原生适配器，也未完成原文清单、读取/删除、持续开启提示、诊断包排除检查和真实平台端到端验证。短时许可在有效窗口内可供同一 task/kind 重用，不宣称为单次令牌。四目标编译不是 macOS/Windows 原生运行证据。本批未提交、推送或合并。
