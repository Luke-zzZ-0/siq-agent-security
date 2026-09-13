# M71：macOS teardown 编排

日期：2026-09-12。候选：codex/personal-macos-stop-recovery 基于 274ed97 的 M68–M71 增量。规格 §3.11.33；teardown --confirm-teardown。无新增持久合同。

macOS 分支在一个生命周期锁内完成正常停止与注销，拒绝 pending 切换。停止未确认不注销；注销失败重试时复用已停止状态。链接缺失只走注销的域缺席复验，不恢复链接，不接管仍加载任务。配置、身份和历史保留，智能体钩子不卸载；再次 setup 可复用已有配置。

## 验证

- Go vet、全量、CLI race 与四目标构建通过；gofmt、git diff --check 通过。
- 8 项临时状态/真实签名与链接、模拟 launchctl 集成场景：运行中退出、已闲置、已移除、停止失败、卸载失败后恢复、查询失败、缺链接但仍加载、未知文件。
- 运行场景持有真实 daemon Writer，模拟 stop 后释放；停止失败不调用 bootout；卸载重试不会重复停止。成功重复调用通过。各场景逐字节对比操作前全部状态文件，原文件保持。
- 最终 linux/arm64 候选 TestNativeSetupAndReuse 通过（2.92 秒），覆盖既有 Linux setup、复用、ui、自启链接、teardown 及重装；仅隔离 runtime 服务。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | e68038dbacf038c7c5376643e71d3c69d685cbf6707b586a1f07e992f941b88f |
| linux/amd64 | 1fb817936a1993873f82774bc51eff7a61c51a61ee1e2c80408c18fbb195e077 |
| darwin/arm64 | c6c50414158d93ea4cfe347ea9accaa25ceb722bf8e84ee6f682d7697f98249f |
| windows/amd64 | 9e654cf7bded71f486dcc45abb38e244da9710559257c7e0f027d4dd4d0b1942 |

模拟不证明 macOS 信号排空、launchctl 兼容或完整原生重装。Windows 生命周期、正式制品及真实 OS/平台综合验收待完成；UX-003 与总体目标保持进行中。本批本地落盘，不改变 PR #31。
