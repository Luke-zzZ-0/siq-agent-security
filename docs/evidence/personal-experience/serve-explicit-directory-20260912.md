# M72：serve 显式状态目录

日期：2026-09-12。候选：codex/personal-macos-stop-recovery 基于 274ed97 的 M68–M72 增量。规格 §3.11.34；serve --state-dir。无新增持久合同。

[Microsoft ExecAction 文档](https://learn.microsoft.com/en-us/windows/win32/taskschd/execaction)指出任务引擎会缓存动作中使用的环境变量值，因此 Windows 后台实例需要显式目录参数。新参数直接指定当前 serve 的状态目录，不修改环境，不引入 shell 包装器。原环境默认路径行为保留。

## 验证

- Go vet、全量、CLI race、四目标构建通过，gofmt/diff 检查通过。
- 选择器正向验证显式参数优先和缺省环境兼容；空值、相对路径、非规范、目录链接、普通文件、缺失目录拒绝；无效 CLI 不创建环境目录，不改变环境。
- 最终 Linux arm64 二进制真实子进程测试通过（0.07 秒）：两个新旧环境变量均指向另一个不存在目录，serve --state-dir 选择已初始化实例，目录健康通过、所选 Writer 存在、另一目录未创建，SIGINT 正常退出。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | 5c6e77ec5c4658150033d974721ca66f9156fcb9308366b1d64a2502e156e068 |
| linux/amd64 | ed1a33e188a022636543917cf04e9cedbd3d88273192c7126e27f53199f39c68 |
| darwin/arm64 | d55ff3296d39e937d17cf190b60f8a28ede54c3038d752e4b01975787ca5e4c2 |
| windows/amd64 | 46d97884ca94868300bc24b3908621308b1fae5aae390312cb47ce8bedef444d |

此批仅提供计划任务所需目录绑定能力，不包含 Windows XML/身份/系统注册/启动或 Windows 实机证据；下一步继续实现用户级任务配置和生命周期。个人/LAN 完整目标保持进行中。
