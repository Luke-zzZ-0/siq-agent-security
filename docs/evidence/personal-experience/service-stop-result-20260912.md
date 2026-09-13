# M85：本次排空结果与后台收尾

日期：2026-09-12。规格 §3.11.47，新增 local-service-stop-result/v1；本地后继增量基于 274ed97。

服务在已接受当前运行的签名停止请求后，完成 HTTP handler 排空、刷新协程退出和运行检查收尾，再持主 Writer 排他写入结果。结果绑定完整签名接受记录摘要、boot/目录、完成时间及 drained/drain_failed。无签名停止请求的信号退出不伪造结果。

运行检查超出宽限期仍等待实际完成，保留超时错误并记录失败，避免活跃后台写入尚未结束就释放 Writer。结果记录失败也返回错误。drained 不是进程已退出，写入时仍持 Writer；客户端须后续确认 Writer 释放。

## 验证

- Go 全量/vet、localcontrol race（1.057 秒）、state race（4.184 秒）、CLI race（7.528 秒）通过。
- 状态层覆盖缺少接受记录、缺少 Writer、逆序时间、成功/失败状态、幂等重试、冲突与未知文件拒绝；原记录不覆盖。
- 收尾测试证明超时后等待活动检查结束且保留超时错误，正常收尾仅调用一次。
- 共享 Go/Python 样例核对签名、完整接受记录摘要、时间顺序和字段关系；165 项合同测试与 Ruff 通过。
- 四目标构建通过；最终候选隔离 Linux 直接 HTTP 与完整 stop-request CLI 两条真实退出测试通过（共 0.219 秒），新增核对签名 drained 结果，退出码零且主 Writer 可重新获取。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | 00ba367cc12ec3cf54017520d70b5689997c63c4836e8ac766ba2b14a56537f3 |
| linux/amd64 | cd968e0d44745d84f750dd21a2e79b0dfb8066515fb595ed760e889afd635114 |
| darwin/arm64 | 019d0f54bfc13edda2406254d71e5f5082d1324f39d132fcd11e2b12217022c5 |
| windows/amd64 | 70b280fdd3da1e5ed95efa252d3d82b0dc8b328bc9f90756030cec6deb43dabe |

## 后续

下一步最终停止确认客户端须同时检查当前接受/排空结果和 Writer，Windows 还须对应任务状态；单独看到 drained 或 HTTP 202 不能报告全部停止完成。没有 macOS/Windows 原生后台退出证据，不提升 UX-003 验收状态。功能增量未提交、推送或合并。
