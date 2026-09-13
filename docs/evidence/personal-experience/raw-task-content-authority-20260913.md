# M114 逐任务原文采集授权与撤销核心

日期：2026-09-13；任务：UX-013；规格：§3.12.23、ADR-048。

本批在 M113 独立密文仓之上新增签名授权层。`local-raw-task-content-grant/v1` 仅保存任务与操作者 sha256 引用，并固定可采集内容种类、采集窗口、密文保留期及单条规范明文上限。授权窗口限制为 1 分钟至 24 小时，保留期限制为 1 小时至当前仓配置上限，单条上限不超过 1 MiB。授权使用现有本机 Ed25519 身份签名；AES-256-GCM 密钥仍独立保存。

`Store.Write` 已改为包内 `write`，生产调用方只能使用 `Authority.Capture`。每次采集都会在同一互斥区内重新读取并验签授权，检查任务摘要、内容种类、当前时间、保留期、单条大小及撤销墓碑，然后才写入密文。授权目录的只读打开不创建任何文件；显式初始化目录也不会签发授权。

`local-raw-task-content-revocation/v1` 是不可变终态墓碑，绑定授权 ID、完整授权签名和撤销操作者摘要。撤销请求以预期授权签名作 CAS：错误值返回冲突，相同值重试返回首份记录。撤销与采集串行，撤销完成后授权不再产生密文；此前密文仍按原保留期读取、删除或清理，撤销不修改回执、效果证据或追溯包。

验证通过：

- `go test ./internal/rawcontent`；覆盖默认无副作用、错误状态目录、任务/种类/时间/大小边界、授权与撤销篡改、错误 CAS、幂等撤销、撤销后拒绝采集以及既有密文保留。
- `go test -race ./internal/rawcontent`。
- `go test ./...` 与 `go vet ./...`（`apps/agentshield`）。
- `local-raw-task-content-grant.v1.schema.json`、`local-raw-task-content-revocation.v1.schema.json` 的固定 Go 样例通过 Python Draft 7 校验；Python 使用固定种子独立验证两份 Ed25519 签名及 Grant/Revocation 绑定。合同测试共 178 项，Ruff 通过。
- `CGO_ENABLED=0` 下 `linux/amd64`、`linux/arm64`、`darwin/arm64`、`windows/amd64` 四目标编译通过。
- `gofmt -l internal/rawcontent` 与 `git diff --check` 无输出。

边界：本批只有安全核心与跨语言合同，服务端尚未初始化该组件，也没有管理授权、采集、读取或删除 API/UI，因此默认产品运行仍不采集原文。四目标编译不是 macOS/Windows 原生运行证据；没有增加任何平台支持声明。诊断包排除检查和真实任务采集验收仍待后续批次完成。本批未提交、推送或合并。
