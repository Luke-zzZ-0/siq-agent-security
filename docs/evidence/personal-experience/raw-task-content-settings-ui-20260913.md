# M119 原文仓持续状态与隐私设置界面

日期：2026-09-13；任务：UX-012、UX-013；规格：§3.12.28、ADR-048。

本批把 M115 的签名启用状态和 M118 的到期清理能力接入个人版嵌入式管理界面。所有页面读取继续经过管理会话；浏览器只严格检查响应合同，不在前端复制服务端签名信任逻辑。

可见行为：

- 本地控制台顶栏持续显示原文仓 `正在读取`、`关闭`、`已启用 · 按任务授权`、`状态异常` 或 `不可用`。页面加载后每 30 秒、窗口重新获得焦点及设置变更后刷新。
- `ready` 文案明确表示仓已启用但默认采集仍关闭，不能推导任何任务正在保存原文。
- 设置页首次启用前展示保留期、磁盘上限、当前人工身份和显式确认；服务端返回的固定限制与签名启用记录经过精确字段、范围和字段关系检查后才展示。
- 到期清理要求独立确认，只调用 `confirm_expired_only=true` 的管理接口，并说明任务回执和追溯记录不受影响。
- 响应格式、状态、密钥或签名异常时撤下旧的可操作状态，不自动覆盖、重建或把异常解释为已启用。
- 原文状态、Activation、限制和签名不写入 localStorage/sessionStorage。界面不显示原文记录或明文。

真实嵌入式浏览器旅程使用临时状态目录与实际 Go daemon，验证新安装默认关闭、管理配对、显式启用 7 天/256 MiB、顶栏即时同步、零记录到期清理、刷新后的签名状态恢复、伪造 `default_capture=true` 响应失败关闭、浏览器存储边界及 390px 无横向溢出。结果为 7 项通过且无 page error：

- [结构化结果](raw-task-content-settings-20260913/browser/result.json)
- [完整设置面板](raw-task-content-settings-20260913/browser/raw-content-panel-desktop.png)
- [桌面控制台](raw-task-content-settings-20260913/browser/raw-content-ready-desktop.png)
- [移动端设置面板](raw-task-content-settings-20260913/browser/raw-content-ready-mobile.png)

验证通过：

- Web：20 个测试文件、66 项 Vitest；个人版和企业版两种 TypeScript/Vite 构建。
- Browser：`raw-content-settings-browser-smoke.py` 对真实 embed/daemon 执行上述旅程；脚本 Ruff 通过。
- Go：`go test ./...`、`go vet ./...`；`rawcontent`、`server`、`ui` race 通过。
- 合同：196 项 Python Draft 7 合同测试和 Ruff 通过。
- `CGO_ENABLED=0` 四目标 `go build -trimpath ./cmd/agentshield`：`linux/amd64` `6b2455f5dc8b7bb3dcbdefb113cfea3208090152c86d655a212ff642e44cdd84`、`linux/arm64` `60e827c2870da5c54aec8ecd837989551ed74fdc1bf914ecdc7ec85a25e72590`、`darwin/arm64` `43172ba87997d34a254e8eb8c3223538668c4711dbe4b831c1833f600f991869`、`windows/amd64` `16c7a1f5935d280ea45e0f10b40c4603c0e093faeda54f1cdcfff8c017b5757f`。
- `git diff --check` 无输出。

边界：本批没有开放任务级 Grant/Revoke、记录清单、明文读取或单条删除界面；自动定时清理、诊断包排除检查和 Hermes/OpenClaw/WorkBuddy 原生采集仍待完成。Chromium 旅程证明本地产品入口和服务协议联通，不构成任一智能体平台原生采集证据；四目标编译也不构成 macOS/Windows 实机验收。本批未提交、推送或合并。
