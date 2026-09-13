# M70：macOS setup 编排

日期：2026-09-12。候选：codex/personal-macos-stop-recovery 基于 274ed97 的 M68–M70 本地增量。规格 §3.11.32，setup --confirm-setup [--port N] [--open-ui]。无新增持久合同。

setup 的 macOS 分支先验证当前 GUI 域，拒绝 Linux 专属 runtime 模式。未健康实例经不覆盖初始化、归属注册、加载启动和最后目录健康检查后输出管理 URL；健康实例不重写初始化或注册，而由启动入口核对归属并复用。失败保留已完成阶段，不继续执行。配对/权限仍由用户明确处理。

## 验证

- Go vet、全量、CLI race 通过。帮助文本同步后 CLI 测试与四目标构建再验证通过。
- 11 项模拟编排场景：初次、复用、打开 UI、GUI 域失败、配置失败、初始化失败、注册失败、启动失败、最终健康失败、端口冲突、浏览器失败。断言失败后的阶段不执行，只有最终就绪才输出 URL/打开浏览器。
- 最终 linux/arm64 候选 TestNativeSetupAndReuse 通过（2.84 秒）；隔离 runtime 服务覆盖初始化/复用、ui、登录链接、teardown 与重装复用，未修改生产服务。
- gofmt、git diff --check 通过。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | 6dd9c03d4f9554d29930f1ab88c29ef8d54732179eecca1ab4a13b3c27baf168 |
| linux/amd64 | ad2e6f6a923d613d31e1ddc3d03a4d2362d280b0b6737fa7036b362bd3ef0f7b |
| darwin/arm64 | 622df30cc1e186a73773ca379c76e676840b9f0e779f2f44ab5a4f15482fec0f |
| windows/amd64 | d87d843ee3b80081908597dd84d8e6b0de9f45eaed8bdc4e881b10c4111eca50 |

没有 macOS GUI/launchctl 的原生 setup 证据，模拟动作不证明各系统调用成功；跨编译不代表 Windows 生命周期已支持。macOS teardown 整合、实机、正式安装器/发行路径继续待办。PR #31 候选不变，本批不提交/推送。
