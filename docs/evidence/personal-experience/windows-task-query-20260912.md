# M76：Windows 系统任务只读查询入口

日期：2026-09-12。本地候选基于 274ed97，规格 §3.11.38，命令 `task-query`。无新增持久化合同。

实现通过 GetSystemDirectoryW 定位系统 schtasks.exe，以独立参数执行 /Query /TN 精确签名任务名 /XML，不从 PATH 或 SystemRoot 选择程序。不使用 shell、远程系统或替代账户；15 秒超时、64 KiB 单流输出限制、stderr 非空或进程失败均拒绝，不能据此判定任务缺席。

查询前核对签名记录和源 XML；对 UTF-8 或带 BOM 的 UTF-16LE/BE 做严格转换，完整 XML 核对成功后再次核对本地源，再输出已核对的预期 XML。原始失败输出不转发，配置不匹配时没有成功输出。

接口依据：[Microsoft schtasks query](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/schtasks-query)、[GetSystemDirectoryW](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/nf-sysinfoapi-getsystemdirectoryw)。这些接口文档不代替本机原生验证。

## 验证

- Windows 定向测试、Go vet、全量测试与 CLI race 通过（race 5.156 秒）。首轮测试辅助编码器使用了 ByteOrder 不支持的方法，已改为 PutUint16 后重跑通过。
- 编码正向：UTF-8、有 BOM UTF-8、UTF-16LE/BE，包含中文与补充平面字符；负向：奇数字节、孤立高/低代理、错误代理对、非法 UTF-8、声明与实际编码不一致、空输入及超限。
- 六项带真实临时状态/签名的模拟查询：正常、UTF-16、系统错误、读回提权、本地源在查询中改变、无签名归属。失败均无成功输出，无归属时不调用查询。
- Go Windows 目标编译覆盖系统目录 API 调用；四目标构建通过，未运行 Windows 系统命令。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | 96d6f901bcbddef99672d2b09ebf71c789a3742216aceae3dac91740da2693d8 |
| linux/amd64 | 8c1e9dce05682bfe77d68f009a921fc1db70c8d085d8e94d7cf2e74f7fd58847 |
| darwin/arm64 | bb4d38147eed40d548add499822208263bb36f40a4a74bf99127a83356796d64 |
| windows/amd64 | c8b244c289a7d8fc37102ca9457f5774c916270409d2b7b7e63ca23c7ba93a76 |

## 边界与下一步

模拟测试没有启动 schtasks，没有证明 Windows 当前系统的 XML 序列化、编码、用户会话或本机文件权限符合预期。未知字段和编码拒绝，不通过忽略差异扩大兼容声明。成功仅代表一次完整配置核对，不证明正在运行或健康。

后续补可验证的任务缺席判定、排他注册和正常启停；当前查询失败不能授权覆盖同名任务。UX-003 保持 doing，Windows 原生旅程仍待验收。功能增量仅本地落盘，未提交、推送或合并。
