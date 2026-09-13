> 审查补注（2026-09-13）：本文为建立工作树时的历史基线记录，不能将“无遗漏”解释为全仓库独立验收。当前远端和旧工作树差距需继续复核；N01 独立审查发现验收阻塞，当前进度见 [接续台账](../../../personal-experience-closure-progress-20260913.md)。

# N00：干净可追溯开发基线

2026-09-13，Asia/Shanghai；Linux arm64，Go 1.26.5。

## 基线建立

- 远端 `origin/main` 当前头 `ff99317450784c9563f5b6a2308c98e8262df98d`（与本地 HEAD 一致，无未合并提交）。
- 在隔离工作树 `/tmp/siq-personal-closure` 上创建分支 `glm/personal-closure-20260913-161000`，从 `origin/main` 快进检出。
- **未切换**运行中的 GLM 共享目录 `/home/maoyd/siq/siq-agent-security` 的分支；该目录 407 个未提交文件**未纳入本基线、未提交**（任务书 §3.1 约束）。

## 基线确认：无遗漏修复

在 `ff99317` 上重跑全部既有检查，全部通过，确认合并主线无回归、无遗漏：

- `apps/agentshield`：`gofmt -l .` 无输出；`go vet ./...` exit 0；`go test ./...` 全量通过。
- `apps/web`：Vitest 73 项全部通过（22 文件）。
- 研究脚本 unittest、站点链接/发布身份、Actions 固定引用、Mermaid vendor 校验在上一批整合中已验证，本基线为同一主线，无新增改动。

## 环境版本记录

| 工具 | 版本 |
| --- | --- |
| Go | 1.26.5 linux/arm64 |
| Node | v22.22.2 |
| npm | 10.9.7 |
| Python | 3.13.12 |
| git | 2.43.0 |

## 本批范围

仅建立可追溯基线，**不含任何实现改动**。下一批为 N01（统一状态格式兼容、迁移、回滚保护）。
