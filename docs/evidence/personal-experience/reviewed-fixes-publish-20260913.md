# 阶段审查修复提交验证（2026-09-13）

用户授权：“请将你的修复提交到远端”。使用隔离工作树与独立分支 `codex/reviewed-stage-fixes-20260913` 整理，不切换、暂存或改写 GLM 正在开发的共享工作树。

## 提交范围

- 远端基线：`b6f186d`；保留 main 已更新的中英文 README。
- 前置历史：通过合并 `274ed97` 引入已提交的 M48–M67。
- 合同基础：`b581cbe`，已完成阶段的 schema、样例和合同验证。
- 实现基础：`14c26cf`，截至 M130 的依赖实现，以及此前已复验的 M124–M128 适配器与归因修复。该提交包含 GLM 的已完成基础，不能解释为全部由本次审查编写或全部产品目标已验收。
- 后续独立修复提交：M129/M130 场景效果限制、场景复用冲突、通知失败退避和通知输出丢弃，包含正负向回归与规格更新。
- 排除 M131 Git 来源导入及之后开发；本批不合并 main、不发布制品、不重启用户服务。

## 最终分支复验

以下检查均针对本次隔离分支重新执行，日志与构建摘要见 [验证清单](reviewed-fixes-publish-20260913/verification.json)。

| 检查 | 结果 |
| --- | --- |
| `go test -count=1 ./...` | 全模块通过 |
| `go vet ./...`、`gofmt -l`、`git diff --check` | 通过 |
| grant、receipt、notify、runtimeidentity、server 五包 `go test -race` | 通过 |
| Python schema 合同 | 197 项通过 |
| Hermes 适配器 pytest | 109 项通过 |
| OpenClaw managed-bridge Node 测试 | 47 项通过 |
| `node scripts/test-openclaw-adapter.cjs` | 通过 |
| `npm ci --ignore-scripts`、`npm run test` | 依赖安装成功，21 文件 / 70 测试通过 |
| `npm run build`、`npm run build:local` | 通过，重新生成的本地 embed 与阶段快照一致 |
| CGO 关闭的 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 构建 | 全部通过 |

适配器源文件和内嵌文件的一致性由 Go adapterinstall 测试验证。此次未重复调用真实智能体或桌面通知；原生 OpenClaw 证据见 [前次复验](stage-review-fixes-20260913.md)。跨平台构建不能代替 Windows/macOS 实机验收；可信 Skill 执行来源、WorkBuddy 和 LAN 团队目标保持原有未完成状态。

历史修复摘要见 [M129/M130 复验记录](stage-fixes-m129-m130-20260913.md)。其中制品摘要来自当时快照，包含当时保留但未验收的 M131 内容；本次交付明确排除 M131，使用本目录的独立构建摘要。
