# M68：macOS 停止及退出复验

日期：2026-09-12。候选：codex/personal-macos-stop-recovery 基于 274ed97 的本地增量。规格 §3.11.30；CLI launch-agent-stop --confirm-stop。独立于等待审查的 PR #31，无远端变更。

停止入口验证当前 GUI 域、源签名、用户目录链接和完整已加载配置，仅向正 PID 的归属实例发 stop。主动停止要求 PID 消失、同一已验证 XML 响应 LastExitStatus=0，随后持有主 Writer 再读回；异常/缺失退出状态、失联和锁冲突不报告成功。不 bootout、disable、删除文件或直接按 PID 杀进程。最初不存在/闲置时不宣称此前进程正常退出。

stop label 接口依据 [Apple 历史 launchctl 源码](https://github.com/apple-oss-distributions/launchd/blob/main/support/launchctl.c) start_stop_remove_cmd。当前 macOS 接口与 serve 信号排空仍需实机验证；模拟控制器不构成原生证据。

## 验证

- Go vet、全量测试、CLI race 通过。
- 10 个隔离模拟场景通过：正常停止、已闲置、未加载、未知配置、stop 失败、持续运行、异常退出、缺退出状态、Writer 冲突、任务消失。确认参数缺失/无效/多余参数拒绝。
- 失败后签名源保持有效；操作释放其 Writer，不接管其他写者；未知配置不调用 stop。
- 四目标交叉构建通过；gofmt、git diff --check 通过。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | fd7133da376411c979e2924822f2738d0b930f48c11d4741ef735a119a150b1a |
| linux/amd64 | e2264a7bc3c0bef227fccd5c4fc7e0b63c412a8566582a769b761cb691894b22 |
| darwin/arm64 | ad620243f732cbd8b08f6d0238f021ad58ea81d3d42689884015e341dcfd9cc5 |
| windows/amd64 | f012c8d41a976f966d61768e7d4ea2f14d80f3540aeec2057e07f635312c92c2 |

没有 macOS 原生停止、注销或完整安装体验验收，不计 UX-003 完成。后续继续配置注销/失败恢复整合及跨 OS 生命周期；完整个人/LAN 目标保持进行中。
