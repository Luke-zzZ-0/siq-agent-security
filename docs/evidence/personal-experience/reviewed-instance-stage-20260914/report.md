# 阶段提交独立验收：实例目录保护与配置还原

本候选从最新 origin/main `412d3b7` 建立独立分支，提取 GLM 的实例发现/配置还原测试和原始权限位修复，加入 Codex 已复核的 R1 配置根身份绑定。GLM 工作树和未提交内容保持原样。此报告记录提交前独立验证；最终提交身份及 CI/合并状态以关联 PR 为准。

## 接受范围

- 预览绑定实例目录的进程内文件系统身份，目录移动、原路径重建或链接替换后拒绝旧计划；重新预览可正常安装。
- 卸载时保留用户配置字段并恢复首次接入记录的权限位；一实例卸载不修改另一实例。
- 中文/空格 profile、非默认路径、移动/删除发现，以及安装/卸载的陈旧计划回归。
- 可重跑的 Linux Hermes 隔离旅程脚本，使用公共 CLI、合成模型、本地测试文件；不调用收费模型，不修改用户生产 profile。
- Windows 测试只略过 POSIX 权限位等价断言，继续验证配置内容和归属；未声称 Windows/macOS 原生通过。

公开合同与恢复载荷不变。进程内 FileInfo 不持久化，不能把恢复载荷当新预览应用；既有已开始事务恢复仍按归属和前后像处理。本修复不宣称阻挡同 UID 在校验后的任意文件系统替换。

## 验证

| 检查 | 结果 |
| --- | --- |
| 权限位回归运行于原实现 | 按预期失败：surgical restore lost the original mode，见 before-mode-fix.log |
| gofmt / go vet ./... | 通过 |
| go test ./... -count=1 | 39 个含测试包通过 |
| go test -race ./internal/adapterinstall ./internal/hermeshome -count=1 | 通过 |
| CGO_ENABLED=0 四目标构建 | linux amd64/arm64、darwin arm64、windows amd64 通过 |
| Linux ARM64 真实 Hermes CLI 旅程 | 13/13 通过，见 candidate-journey-report.json |
| git diff --check | 通过 |

原生旅程用候选二进制覆盖发现、预览无副作用、接入、允许读、越权写执行前拒绝、服务离线阻断、同端口重启、回执链、卸载身份撤销和配置语义还原。Hermes CLI 自身规范化 YAML，因此原生旅程检查语义和安全权限；无需原生 CLI 的 Go 用例验证字节及原始权限位。浏览器审批、OS 隔离、Windows/macOS 实机不包含在此报告中。

复跑：先在 apps/agentshield 执行 `go build -o /tmp/siq-reviewed-native ./cmd/agentshield`，再从仓库根目录执行 `python3 scripts/personal-experience/config-restore-journey-smoke.py --hermes-cli /path/to/hermes --binary /tmp/siq-reviewed-native --out /tmp/siq-reviewed-journey.json`。首次本机尝试误选 x86_64 交叉制品，启动即 Exec format error，无运行旅程；改用 ARM64 制品后通过。候选脚本与二进制摘要均记录在最终旅程报告中。

## 本次保留待验收的内容

- N02：任务书要求真实生产公网成功路径；现有报告仍只有受控失败及 fixture 成功，不在本次启用生产 Git。
- N03 与 R2：R2 修复留在 GLM 来源调度实现中，需随 N03 独立候选审查合同、调度与前端后提交；本次 main 不包含该新存储入口。
- N04 其他诊断/能力矩阵/安装入口提示：仍在 GLM 树，未借本次测试扩大验收声明。
- N05：安装内容与自报元数据匹配尚不等于证明本次调用的真实 Skill 来源，保留独立安全验收。
- N06/N08：跨会话审批消费放宽了 intent/task 匹配，需独立核查审批对象、重试副作用及持久化失败语义；整包不随本批合并。
- N07/N09 和局域网团队目标均未由此关闭。
