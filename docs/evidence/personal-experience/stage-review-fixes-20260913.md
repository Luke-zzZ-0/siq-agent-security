# M124–M128 阶段审查修复（2026-09-13）

范围：根据用户授权修复已完成阶段的审查问题；不接管另一窗口正在开发的后继阶段。基线为 `codex/personal-macos-stop-recovery`、HEAD `274ed9791df24d5b061c528f1a3769879f5e5ba2` 上的当时工作区快照，包含既有未提交成果，不能把 HEAD 单独当作本批源码身份。先在 `/tmp/siq-stage-review-20260913` 隔离验证，再按文件基线摘要核对并应用增量。

## 修复结果

| 问题 | 当前行为与验证 |
| --- | --- |
| OpenClaw 安装后原生 CLI 拒绝配置 | 停止注入不受支持的 `security.installPolicy`；只迁移归属记录与完整内容匹配的历史策略，保留未知配置。安装、重装、卸载与迁移测试覆盖；盘点不再误报原生安装门禁 |
| 托管 bearer 可能被发送至外部地址或重定向目标 | 仅接受显式端口 HTTP loopback，固定 localhost 为 127.0.0.1，禁止 URL 用户信息、路径、查询、片段及重定向；凭据文件有界读取，拒绝符号链接 |
| managed 在 warn/audit 下验证失败可能放行 | 配置损坏不回退 legacy；身份、会话、登记响应或决策引用无效时所有模式硬拒绝；普通非托管 advisory 语义保留 |
| 输出原文关联可重复使用，hold 尚未最终批准也可能采集 | Hermes/OpenClaw 关联消费一次并保留失效标记；重复 pre 使旧关联失效；OpenClaw hold 仅在最终执行复验成功后具有输出资格；Hermes MCP 结果也要求有效允许关联 |
| Hermes 4 项回归失败 | 正向夹具补真实允许决策的 action_id；补缺引用、重复 pre/post、禁止阻断结果采集测试，不通过放宽输出归属掩盖失败 |
| SkillClaim 精确匹配被当作执行来源验证 | Store 精确匹配返回 unknown，复制已批准 Skill 元数据不能满足 verified 门禁。保留元数据、漂移提示和门禁框架；可信执行绑定未完成，强制开关仍默认关闭 |

规格已同步至 `docs/agentshield-dev-spec-v1.md` §3.12.33 / §4.1，适配器 README、盘点合同样例与开发台账同步修正。M124–M128 原始报告作为历史证据保留，不把后续修复写成早期测试已覆盖。

## 验证

- OpenClaw 托管桥：47 项通过，包括三模式身份/决策失败、坏配置、外部地址、重定向、过期、缺引用与重复输出等负向路径。
- OpenClaw 既有 hook/审批最终复验测试通过。
- Hermes：109 项通过；源适配器和 Go embed 副本逐字节一致。
- Go：全模块 `go test ./...`、`go vet ./...`、`gofmt -l .` 通过。四目标构建通过：linux/amd64、linux/arm64、darwin/arm64、windows/amd64。
- Python 合同：197 项通过，含更新后的 Go inventory 样例。
- [修复后 OpenClaw 原生会话](openclaw-managed-native-smoke-reviewed-20260913.json)：真实 OpenClaw 2026.5.12，Linux arm64，公共 `agent --local`，17 项检查通过，3 条签名回执、2 条原文记录。夹具仅增加合成模型/工具配置，不再删除安装器字段；身份撤销后新调用被拒绝。
- [核验清单与文件摘要](stage-review-verification-20260913.json)；[M127 原始 JSON](openclaw-managed-native-smoke-m127-original-20260913.json) 单独存档。

合入工作区时检测到另一窗口追加 M129 台账和 `state.go` 通知配置，已保留这些增量，仅应用本批对应位置的修正。合入后再次运行 Go 全模块测试通过、`git diff --check` 干净、适配器源/embed 一致；这次兼容回归不代表对后继阶段的功能验收。隔离快照摘要与合入后 state.go 摘要在核验清单中分别记录。

## 未关闭的验收条件

以上不是三系统九种组合整体完成。macOS/Windows 原生生命周期、WorkBuddy、真实用户审批恢复、可信 Skill 执行来源、正式发行安装仍待验收；原生 OpenClaw 安装入口拦截尚未证明。原生会话使用合成模型与隔离 profile，不声称 OS/网络隔离或真实用户工作流通过。未改用户后台服务，未提交、推送或合并。

下一步应设计由可信运行时签发、绑定具体调用与 Skill 内容的执行来源凭据，并验证跨 Skill 冒用、重放、换版、撤销；不能仅增加调用方自报字段。另一窗口的后续阶段先独立完成，再做增量审查。
