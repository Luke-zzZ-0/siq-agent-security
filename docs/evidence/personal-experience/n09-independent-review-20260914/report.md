# N09-A/B 独立复核与修复（2026-09-14）

结论：保留 Linux/Hermes 与 Linux/OpenClaw 的部分真实观察；撤销两个平台 J1–J11 整体 complete_acceptance 推导。N09 未关闭，团队 T01–T06 仍遵守前置验收门槛。

## 已复现的问题

1. 原校验器仅核对报告文件摘要，不读取 passed、嵌套失败、检查数量或二进制身份。给失败报告重新计算摘要仍可被接受；原实现归档为 original-checker.py.txt，仅作历史审查证据。
2. 路径使用字符串 startswith，docs/evidence-escape 等相似前缀能越界；符号链接也未被禁止。
3. native/controlled_start 被强制等同 complete_acceptance；同一份报告被填入所有旅程行，OpenClaw J2/J6 明知未演练仍标为完成。
4. C6b 创建直接 API hold 再批准消费，没有原生工具批准后执行和副作用次数证据；J7 身份与签名链不等于可信 Skill 调用归属。
5. L3 的 11 条检查中存在重复名称，不接受其独立通过数量。原报告不改写。
6. OpenClaw runner 用 all(已有断言) 计算通过：空检查或异常中断后留下的全部通过前缀可能生成绿色报告。

## 已交付的修复

- 版本化 v2 schema、严格 JSON/路径/摘要/二进制/环境检查、逐行具名 coverage。完整验收必须有显式 acceptance_scope，仍需人工核对场景语义。
- matrix.json 是本次唯一修订矩阵。两个 original-matrix.json 是有误历史材料，不可用于发布判断。引用的原始报告字节及 fe03e7e0 二进制身份保持不变；它们不是最新 main 的独立重测。
- 失败、空/重复检查、嵌套失败、布尔计数、跨平台挪用、缺摘要、虚构检查、假完整验收、非法 JSON、路径前缀/穿越/符号链接均有离线负向回归。
- OpenClaw 的完整检查集合与失败闭合函数进入共享 helper。runtime-completion.patch 已精准应用到 GLM 工作树，runner 要求 run 正常走完且 11 个具名检查全部成功。runner 依赖 GLM 尚未合并 API，本批不将整份 runner 搬进 main；精确补丁可供后续候选审查。

## 验证和边界

本批运行离线 unittest（7 个测试方法，含多组对抗子例）、schema 样例校验、修订矩阵校验、Python 编译与 diff 检查。未运行收费模型、原生 OpenClaw 旅程、Windows/macOS，也未把旧数据改为新候选证据。CI 加入三 OS 的离线校验器回归，不代表三 OS 智能体实测。

## 提交与后续顺序

1. 先合并 sunbo PR #40 的 Windows 构建/精确任务生命周期适配；保留 Windows 全量失败与 Issue #39。
2. 独立提交本批合同、校验器和审查证据；与 GLM 产品改动分开验收。
3. N03 定时新版检查须携同状态兼容屏障一起审查；旧/未知状态写入必须零副作用拒绝，不能只交 happy path。
4. N05 需要可信运行时将已安装 Skill 的不可变身份绑定到实际工具调用，用户字段加磁盘摘要校验只能作声明，不足以证明因果归属。
5. N06 审批恢复须验证同一权限版本下不同 intent/task 仍不能互换批准，参数变化重审、并发消费只执行一次、原生工具真实恢复。
6. N08 加载竞态需覆盖卸载/会话切换/重连，旧响应不能覆盖新状态；UI effective 始终后端读回。
7. 补齐 N09 产品安装、发现、Skill 全生命周期、可信归属、隐私流程；Windows 与 macOS 分别由 sunbo/Luke 的固定候选实机结果补充，不能从本次 Linux 数据外推。

本批不修改已发布比赛快照，不变更分支保护，不覆盖 GLM 其他未提交成果。

## Windows CI 回归补充

首轮新校验器的 Windows CI 在历史报告摘要处拒绝；同轮合成负向用例通过。原因是 Git core.autocrlf 改写了新归档 JSON 的换行，而 Windows 专属目录既有 -text 规则没有覆盖这些 Linux 报告。为本批三个证据目录添加精确 -text 属性，保留原字节；没有修改报告摘要或放宽校验。最终提交需重新完成三 OS CI。
