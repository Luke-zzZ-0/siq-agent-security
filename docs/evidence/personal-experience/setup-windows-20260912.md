# M89：Windows 一步初始化

日期：2026-09-12。规格 §3.11.51，本地候选基于 274ed97；未提交或推送。

`setup --confirm-setup [--port N] [--open-ui]` 在 Windows 先只读检查当前用户 SID/本机 Task Scheduler 根目录，再复用初始化、任务排他注册、启动和目录健康检查。与 macOS 共用阶段编排、端口和健康读取，复用已有健康实例时仍经启动命令核对任务归属。最终健康前不输出就绪，不支持 --runtime，不添加登录触发器。

## 验证

- `go test ./cmd/agentshield -run 'TestSetup(LaunchAgent|WindowsTask)StagesAndReuse' -count=1`：两平台各 11 项模拟场景通过。覆盖首次、复用、打开浏览器、预检失败、配置失败、初始化失败、注册失败、启动失败、最终健康失败、显式端口冲突、浏览器失败；断言阶段顺序和就绪输出。
- `go vet ./... && go test ./... && go test -race ./cmd/agentshield` 通过；CLI race 7.187 秒。随后仅更新 CLI 帮助平台列表并重建最终候选。
- 四目标最终构建通过，格式和 git diff --check 通过。

| 目标 | SHA-256 |
| --- | --- |
| linux/arm64 | 6f9ffb65b760f14a0accfdb47ffdf7a7ed82f527c2ea2b52650dbe7f14b07227 |
| linux/amd64 | 27a671221ee39384b398b8c39ca843262655c7aed2a8ea3d74e83ddcaca40cb6 |
| darwin/arm64 | 38e794c2ab6c3d19fbf91e6efb9b6618b1b3158c2862a8a38d2dbf5b961aae7f |
| windows/amd64 | 57efb7e81682b08298a0777a60a75b45f172899ca58e9c3d24d091bee1cb2689 |

## 边界与后续

阶段回调为模拟，没有 Windows/macOS 实机；新增预检 PowerShell 未在宿主解析或执行，COM 可用性、完整安装/启动/浏览器旅程均待原生验收。编译不等于原生支持，当前不是正式安装包，也未实现 Windows 登录自启。下一步 Windows teardown，UX-003 仍未完成。
