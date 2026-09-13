# M123 Hermes 原文采集运行时桥

日期：2026-09-13；任务：UX-013；规格：§3.12.32、ADR-048。

本批补齐薄适配器无法安全持有服务端 task ID 与管理端 Grant 的协议缺口。新增 Runtime Identity 专用 `POST /v1/raw-task-content/native-captures`：请求只有 platform、agent_id、原生 session、kind 和结构化 fields。服务端从已认证身份与签名 Binding 恢复任务范围，完整验证原文仓与全部 Grant/Revocation，仅在当前任务和 kind 恰好有一份 active Grant 时，内部签发 10 秒许可并立即走既有许可采集路径。缺失授权拒绝，重叠授权冲突，适配器不会猜测权限。

Hermes 已管理实例的 pre hook 在工具裁决允许后尝试记录参数，post hook 在既有观察提交后尝试记录实际结果。JSON 参数/结果完整展开为最多 1024 项、最大深度 32 的 JSON Pointer 字段，使 `api_key`、`authorization`、`token` 等嵌套键继续由 Go 原文仓整项过滤。不可表示或越界结构整次不提交，不截断后冒充完整原文。自检身份和旧全局决策凭据不调用原文桥；请求不带 task ID、Binding、Grant、许可或管理凭据。

原文请求是 250ms 上限的本机辅助调用。未启用、未授权、授权冲突、过滤拒绝、不可达或写入错误不改变已完成的工具裁决、原生结果或默认脱敏观察，也不写未签名决策失败记录。

验证通过：

- Hermes 适配器 105 项测试；新增管理实例请求顺序、Runtime Identity bearer、字段展开、权限字段不外泄、失败隔离、旧配置不采集、深度/数量/非法路径拒绝。
- Go `rawcontent`/`server` 的唯一 Grant 解析与原生桥定向测试、定向 race；Go 全量与 vet。
- 新增 native-capture Draft 7 合同及固定 Go 样例；合同测试总集通过，Ruff 通过。
- 安装器 embed 与源码插件逐字一致，gofmt 和 `git diff --check` 无输出。
- `CGO_ENABLED=0` 四目标构建：`linux/amd64` `f3447453decd2abff5747643a72111c8f86bbb02aca326430bf5f568ae311a0b`、`linux/arm64` `ec5520edff4632af0e533b58da1133fab651de92b4422d821a4f4a05d8e605d4`、`darwin/arm64` `0cffb8e7db3e010ff3fc030eec6889bd79918dfb7bbc6b2a4e39e8408a6369bb`、`windows/amd64` `f8c2b9511a3b00f7a54351b2088ad6d220638d4d303c87e5d7082127fcf51b05`。

边界：Go 服务和 Hermes 插件分别完成真实协议逻辑及组件测试，但本批没有用固定 Hermes CLI 版本驱动一次完整原生会话，因此不登记真实平台原文端到端通过。OpenClaw 尚无 Runtime Identity/session 接入，WorkBuddy 仍缺公开阻断钩子与可用测试环境，不能借用 Hermes 结果提升支持。交叉编译不构成 macOS/Windows 实机证据。本批未提交、推送或合并。
