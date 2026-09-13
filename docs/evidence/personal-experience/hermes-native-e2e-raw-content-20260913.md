# M124 证据：真实 Hermes CLI 原生完整会话的原文采集端到端（UX-013）

日期：2026-09-13（本地时间）。平台：Linux arm64 宿主机，真实 Hermes Agent CLI v0.21.0（2026.8.31，git 安装于 `/home/maoyd/siq/hermes-agent`）。

## 本批内容

M123 落地了 Hermes 原文采集运行时桥但明确遗留"未用固定 Hermes CLI 版本驱动一次完整原生会话"。本批补上该验证：运行 `scripts/personal-experience/managed-instance-native-smoke.py`（含 M123 未提交的原文采集断言增量），以公开 `hermes chat --provider custom --model <synthetic> --oneshot` 驱动真实 Hermes Agent 循环与插件生命周期，无注入会话 ID、无自检通道。

## 运行方式

- 守护进程：本仓当前未提交工作树构建（`go build -trimpath -o … ./cmd/agentshield`），linux/arm64 sha256 `dd53474958e06c775c9a31f1723efbedbda4ed9a3e807492ddb39c7285a62752`。
- 冒烟脚本 sha256：`8e586e70ae97b1291062269b3e087bfe55b29d2158defdc9f95e95202cd4d807`（即含 M123 原文增量的当前版本）。
- Hermes CLI 入口 sha256：`4e623fce245c1fe6e70ae0edd3248b376ceaf94d560d63f5881cc7a3415ec609`（`hermes --version` = Hermes Agent v0.21.0）。
- 隔离：临时 `HOME`/`HERMES_HOME`，托管 profile 由产品安装器（adapter plan v3 + 公开 `hermes plugins enable`）配置；`HERMES_BUNDLED_PLUGINS` 指向空目录、项目插件禁用；模型为本地合成 OpenAI 兼容服务。
- 原始报告：[hermes-native-e2e-raw-content-20260913.json](hermes-native-e2e-raw-content-20260913.json)（`passed: true`，2026-09-13T02:01:47Z）。

## 结果（17 项检查全部通过）

安装与授权：托管 preview 不改动宿主、安装使用签发的 Runtime Identity、native enable 保留其他 profile 与用户设置、诊断保持 runtime unverified；产品自检使用独立 Grant/Binding 且不撤销普通身份；环境变量伪造 agent 无效；实例凭据自动注册原生会话。

原文采集（本批核心）：
- 真实原生会话的签名 Binding 自动恢复任务，测试在其中一次允许的读文件之后按该任务创建显式 `local-raw-task-content-grant-create/v1`（kinds=parameters+output，1 小时）。
- 同一会话后续允许调用的参数与结果经 native-captures 协议各产生 1 条密文记录（共 2 条，kinds 恰为 parameters/output）；适配器不持有 task ID、Grant 或许可。
- 管理端 `records/search` + `read` 读回明文，`contains_plaintext=true`；参数明文与实际执行的 `read_file` 路径逐字段一致，输出明文包含实际读到的文件内容 `fixture-visible-company-a`。
- 首次允许调用发生在 Grant 创建之前，正确地未产生记录（默认关闭、无授权不采集的负向证据）。

回执与撤销：允许/拒绝（scope 外写）回执共 5 条 + 产品自检 5 条 = 10 条，回执链 `verify` 通过；撤销 Runtime Identity 后新一轮真实会话全部工具调用被拒，产生 3 条未签名 deny pending 记录，无任何已授权回执，禁止写入未执行。

## 工程校验

- `go vet ./...`、`go test ./...` 全部通过（go1.26.5 linux/arm64）。
- `gofmt -l` 与 `git diff --check` 无输出。
- CGO_ENABLED=0 四目标交叉编译 sha256：
  - linux/amd64 `1d270e68954a9367411e6f3f1487e836841c0873960673456302c835c8852e06`
  - linux/arm64 `81219d5542e3f6f78613dc599089939459a716d0cf9d8472237ddb623906c69d`
  - darwin/arm64 `14a614a308ad38527995f8ee289a7345ae9e971b27e98678bd01f2f00181d16b`
  - windows/amd64 `bda751ff080accf0630c7d12f4e0e6bd5b06df92074ce15f3c076c7b71b2c2fc`

## 登记边界

- 本证据仅覆盖 Hermes（Linux arm64 宿主、合成模型与合成操作者）；macOS/Windows 实机与真实 LLM 供应商不在此列，交叉编译产物不构成其他平台证据。
- 浏览器端任务原文面板（M120）与真实平台采集是两条独立验证线，本批不合并声明。
- OpenClaw 尚无 Runtime Identity/managed 接入，WorkBuddy 仍缺公开阻断钩子与可用测试环境，不能借用 Hermes 结果提升支持。
- 本批仅本地落盘，不提交、不推送。
