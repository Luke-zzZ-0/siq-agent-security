# 主线整合与 N00 基线复核（2026-09-13）

本轮以远端 main `516ec82ed3b12bacab237868c20729a2d2ccd0b2` 为比较基线。GitHub 无开放 PR；55 个本地/origin 引用的分支头全部已在该 main 历史中，包括历史 gh-pages 分支。新增 PR #34 是参考文献材料，不含业务源码。本轮实际需要纳入的是独立工作树尚未提交的 N01 修复。分支不删除，后续 Pages 自动生成提交不作为业务源码追逐合并。

## 旧工作树逐文件检查

原 IDE 工作树 `/home/maoyd/siq/siq-agent-security` 的 HEAD 为 `274ed97`，原始 porcelain 为 407 条；展开未跟踪目录后比较 469 个文件条目，462 条工作区内容与 main 相同，7 个文件不同，均为文档/证据，没有遗漏的业务源码。完整工作区与 index 比较见 `old-worktree-audit.json`，分支快照见 `branch-heads-before.json`。

7 个差异逐字节存入 `historical-worktree-remainder/`：两份 README 缺少 main 已补充分发验证提示；current 文档含早期 N00 描述并缺少历史发布说明；原台账仍宣称未经安全验收的生产 Git 可用；M129/M130 的报告、verification 与 integrated-go.log 是旧工作树应用后的历史补充。保留其历史证据，不反向覆盖当前事实或恢复已关闭的生产 Git。旧工作树的分支、index 和文件均不修改。

## N01 验证继承

本次提交前逐一核对 N01 完成报告中的 144 个源文件指纹，全部一致。已有全量 Go/race/vet、Python 合同 206、Web 74、本地构建、四目标交叉构建及真实 Linux 双二进制/服务验证证据继续有效。跨 OS 原生验收仍归 N07/N09，不把交叉构建当作实机成功。

N00 的分支遗漏与旧工作树差异复核已完成；N01 已完成代码与 Linux 最低门槛。发布结果另行追加，原 N01 报告中的未提交表述是采证时状态，不改写历史验证 JSON。
