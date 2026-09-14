# GLM 未提交成果独立验收与阶段整合（2026-09-14）

## 结论与候选身份

本批可合并 N02 受控传输组件、N03 来源存储/调度/HTTP/Web、N04 接入诊断及 N08 异步界面增量。不能宣称 N02–N09 或个人产品全部完成。生产 Git 入口继续关闭；N05/N06 原实现未通过安全边界审查，未纳入候选。

整合基线 main `df915ca21c25e04b1acbc81956e155f37ffd77e3`；GLM 原分支 `glm/personal-next-20260913-193405`，HEAD `0720730f821373b00bb8bfd5ec870fec147e5746`。源工作树保持原样，未重置、覆盖或批量提交。冻结的 262 个源路径及原始 SHA256 见 [source-inventory.json](source-inventory.json)。候选实际源码摘要见 [candidate-files.json](candidate-files.json)，最终提交身份由承载本目录的 PR HEAD 固定。

`review_candidate` 是经选取并修复后纳入的源路径；`already_in_main` 采用主线较新实现；`held` 尚未交付。原工作树的历史证据、旧台账、模型调用脚本及重建前 bundle 没有整包导入。原 lockfile 的删除式差异未纳入。旧文件仍存在不代表遗漏，后续不得整体合并旧 GLM 分支。

## 接受内容和独立修复

| 范围 | 已验收增量 | 仍未完成 |
| --- | --- | --- |
| N02 | GitHub 元数据解析到固定 commit，再取固定归档的 HTTPS 组件；公网地址固定、TLS、跳转/代理/归档限制测试；生产显式 503、Web 入口禁用 | 真实托管服务成功/失败联测及生产启用 |
| N03 | 显式保存公开 ZIP 来源，签名调度状态、手动/自动协调、有界退避、5 分钟 daemon tick、版本化合同和管理面板 | 真实原生更新全过程、跨 OS 生命周期 |
| N04 | 安装入口提示与接入分层诊断，安装文件不能推导保护；loopback 探测 | 九种 OS/平台实际能力核验 |
| N08 | 页面迟到响应抑制、注销/身份变化/卸载清理，空数组规范化、安装列表忙时重试、原文存储并发回归 | 完整原生用户旅程 |

独立复核修复了：

1. 调度取数期间用户关闭或重新保存来源后，旧结果可覆盖新记录。现在所有写结果路径必须比较取数前捕获的签名；手动/自动 × 关闭/重存四项负向测试保留用户最后写入的完整字节。首次无记录也不能污染后来创建的记录。
2. 保存来源前严格验证旧记录。未来版本、未知字段、损坏签名和非法状态拒绝覆盖，关闭动作同样遵守；兼容回归纳入本批。
3. 诊断曾验证 localhost 后仍拨号原域名。现在仅拨号经验证的 loopback 字面地址，避免再次 DNS 解析越界。
4. UI 不仅阻止同页面旧请求覆盖，也在卸载、注销和身份切换后失效；更新面板取消进行中请求并清除输入。
5. GLM Git 组件未经真实网络验收，不能直接开放。生产 fetch 使用明确关闭的函数，不提供运行时跳过门禁配置；测试缝不能从配置设置。
6. 补齐五份 schema、双端样例和 Python 合同验证。重新生成 Go embed；保留 main 的 Windows 与实例目录身份防护。

浏览器旧脚本在没有实例授权时直接期待确认按钮启用。本批保留按钮禁用断言，连接组件测试明确选择“仅安装连接组件”；未弱化产品权限条件。更新不可用文案断言同步为当前通用来源文案。

## 不接受的安全边界

### N05：安装关联不能证明当前工具由该 Skill 导致

源 `internal/state/skill_attribution.go` 忽略 sessionID，把调用方 claim、已安装内容及 RuntimeGrantWithSeq 复核组合提升为 verified。内容/授权存在证明对象存在，不能证明这一工具调用确实来自该 Skill。调用方可复制同样的关联信息。源 `state` 与 `receipt` 的归属变更整体暂缓；保留主线 unknown/inferred 和拒绝依赖伪造归属的权限行为。

后续必须先定义可信宿主执行上下文和同调用绑定，再实现验证。源目录中的测试不能作为安全结论，不得仅恢复这些断言以获得通过。

### N06：同一身份修订不能代替审批重试链

源 `internal/receipt/action_state.go` 明确不匹配 SessionID，并在 authority revision 为 64 位十六进制且身份一致时放宽 IntentID/TaskID。摘要格式、同一 grant 或身份修订不是跨任务消费许可。没有可信重试血缘就不能将旧审批应用于新任务。相关 engine/action_state/hold_status 与依赖它们的原生 runner 均未导入。

后续需要服务端签发并验证的显式重试关系，绑定主体、实例、授权修订、Skill、工具及最终参数；并发和重启测试必须计算真实副作用次数，不能只数 allow 决策。

## 验证及其范围

- go vet、全量 go test：39 个测试包通过，6 包无测试；日志见 go-test-vet.txt。
- 五个关键包 race 通过：skillimport、skillinstall、server、adapterinstall、rawcontent。
- 前端 84 项测试，TypeScript/企业构建/个人构建通过；Python 合同 211 passed，Ruff 通过。
- Linux amd64/arm64、macOS arm64、Windows amd64 四目标 CGO=0 构建通过，摘要见 cross-builds.json；不是四系统实机验收。
- 真实隔离 daemon + Chromium 的会话、发现、连接组件与移动端流程 28 项通过；未执行真实智能体。见 browser-session/result.json。
- 更新面板 7 个响应替身场景通过，无页面错误；不是 CDN/原生更新证据。见 browser-update-check/verification.json。
- 浏览器使用构建及四目标构建分别记录自身 SHA256；二者构建参数不同，不混用身份。

本机 api.github.com 解析到 198.18.0.29，现有公网地址策略拒绝。未修改解析策略，未伪造真实联网通过。未执行付费模型调用、发行签名或产品发布。

## 合并与接续规则

按合同、后端、界面及证据分提交；在隔离分支上通过 CI 后合入 main。下一任务书必须以实际合并 SHA 为基线。CI 状态和合并结果以 GitHub PR 为准，本地记录不提前声明远端通过。

Windows 的 sunbo 仍在持续开发，macOS 由 Luke 负责；各自 PR 是增量，不等于系统整体验收。N09 保持 partial；T01–T06 不因本批代码合并提前启动。后续工作优先解决 N05/N06 可信绑定，并推进能在 Linux 上独立完成的开发，不空等 Windows/macOS。
