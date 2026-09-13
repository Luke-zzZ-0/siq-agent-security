# M69：macOS 配置注销与中断恢复

日期：2026-09-12。候选：codex/personal-macos-stop-recovery 基于 274ed97 的 M68–M69 本地增量。规格 §3.11.31，CLI launch-agent-unregister --confirm-unregister，无新增持久合同。

完整用户域、XML 配置和文件归属核对后，仅卸载已停止且主 Writer 可获取的精确任务。确认当前域缺席后删除精确注册链接并同步目录；保留状态内签名 plist、配置、密钥与历史。失败保留现场；卸载后尚未删链接、已删链接的状态可重复恢复。链接缺失但任务仍加载不接管，运行进程要求先 stop。

bootout GUI 服务目标形式沿用 [CircleCI 官方 macOS 文档](https://circleci.com/docs/guides/execution-runner/install-machine-runner-3-on-macos/)的卸载接口。实现不套用其其他目录/包管理器操作。无 macOS 环境，不能证明当前系统兼容性或原生注销完成。

## 验证

- Go vet、全量测试、CLI race 通过；gofmt、git diff --check 通过。
- 12 个隔离模拟场景通过：注销、已卸载、已移除链接、运行中、异配置、Writer 冲突、bootout 失败、仍加载、读回失败、未知文件、卸载期间链接被替换、缺链接但仍加载。
- 成功后重复注销通过；失败不移除未知文件，签名源始终保留，bootout 期间主 Writer 必须被持有；确认参数缺失/错误/额外参数拒绝。
- 四目标交叉构建通过，不能代替真实 OS 验收。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | 79ef0739430eb6c77cf5a4433ecfda3ea9d15a4832d964a31a334742d2de84d2 |
| linux/amd64 | 53d3ed69a112ff437865c9a0837424abf64620e9219d3a0ea2be10a2916d672a |
| darwin/arm64 | 2a71a012b94f965e788b18ac2464fd5665dd92c895afdd18211be32f7f196168 |
| windows/amd64 | 9a37a31bc703b8e4aff9914a22f8fa8077a358d0e74d2a7c50fa567ee6a6bf65 |

后续：macOS setup/teardown 整合与真实完整生命周期、Windows 生命周期。PR #31 仍保持 M48–M67 原候选，本批不提交/推送；目标保持进行中。
