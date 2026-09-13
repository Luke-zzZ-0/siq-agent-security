# M90：Windows 保留数据退出

日期：2026-09-12。规格 §3.11.52，本地候选基于 274ed97；未提交或推送。

`teardown --confirm-teardown` 在 Windows 持生命周期锁贯穿停止与注销，复用签名停止、任务运行状态和注销核心。停止未确认不注销；已缺席跳过停止，但仍取得主 Writer 并复验缺席与本地归属。保留程序、配置、密钥和历史，智能体钩子不卸载。

## 验证

- `go test ./cmd/agentshield -run TestWindowsTeardownRecovery -count=1`：7 项模拟场景通过，包括空闲、已缺席、查询失败、停止请求失败、删除后响应丢失、主 Writer 忙、生命周期 Writer 忙。核对删除与请求次数、归属配置保留、两类锁释放。删除响应丢失后重试成功且不再删除，成功后重复执行幂等。
- `go vet ./... && go test ./... && go test -race ./cmd/agentshield` 通过，CLI race 7.820 秒。
- 四目标构建、格式与 `git diff --check` 通过。

| 目标 | SHA-256 |
| --- | --- |
| linux/arm64 | d5a93eca75daa93c77e71fdeaa34b392e5631f1081efb7c4cbfd3592251fc1ba |
| linux/amd64 | ac6a28c98a7f3fd6956e305091b586376ea2def725266ac826edc4846bba12d1 |
| darwin/arm64 | 792942fb2efb6da3d7efc86856b83fe95a2b687f581410738f036b7138cc52a1 |
| windows/amd64 | fbe1b76edcd5f0ad7e745893b7a768999dee93923e3b6117f12e92afc373f8de |

## 边界

临时状态和真实 Writer/签名归属校验配合模拟系统回调。没有 Windows 宿主，未执行真实 Task Scheduler 删除，未验证原生完整安装/启动/停止/注销旅程，不把交叉编译计为原生支持。运行中正常停止的签名结果路径沿用 M87 单独测试；本批编排测试包含停止失败阻断注销。Windows 登录自启、升级/恢复与实机制品验收仍待完成，UX-003 不标完成。

## M90 补充：运行实例的串联验证

在同一 teardown 编排中增加运行成功与 drain_failed 两项场景，累计 9 项。模拟服务实际持有主 Writer；停止回调创建当前目录绑定的挑战、签名接受记录和排空结果，再释放 Writer。删除回调要求本次 drained 已验签且运行状态已结束；排空失败不得调用删除。成功退出后再次 teardown 不重发停止或重复删除。

`go test ./cmd/agentshield -run TestWindowsTeardownRecovery -count=1` 通过；停止、注销、退出编排联合定向 race 通过（2.292 秒）。补充仅修改测试，生产二进制仍为上表 M90 候选；没有新增原生宿主证据。
