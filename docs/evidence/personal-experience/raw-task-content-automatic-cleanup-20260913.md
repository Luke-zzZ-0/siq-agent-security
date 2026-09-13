# M122 原文到期自动清理

日期：2026-09-13；任务：UX-013；规格：§3.12.31、ADR-048。

本批把既有 expired-only 原文清理接入 `serve` 生命周期。服务维护协程在启动后立即尝试一次，此后每 15 分钟执行；退出时与既有投影刷新协程一同取消并等待结束。未启用原文仓时清理是无副作用空操作，不创建密钥、Activation、权限目录或内容目录。

服务端维护入口每次持原文仓互斥锁重新验证签名 Activation、独立密钥绑定和完整密文目录，再调用既有先认证后删除逻辑。测试覆盖到期记录删除、未到期记录保留、Grant 文件与回执链不变；篡改封套使整次清理失败并保留文件，同时 `/healthz` 继续成功，证明辅助原文仓异常不阻断默认服务。后台不记录任务、记录 ID、正文、密文或密钥信息，失败在下一周期重新验证。

验证通过：

- `go test ./...`、`go vet ./...`（`apps/agentshield`）。
- `go test -race ./internal/server ./cmd/agentshield -run 'TestRawTaskContentAutomaticPurge|TestServeMaintenance' -count=1`。
- 变更文件 gofmt 无输出，`git diff --check` 无输出。
- `CGO_ENABLED=0` 四目标 `go build -trimpath ./cmd/agentshield`：`linux/amd64` `6d874cb260219a40a0c5ed2b3768eca0faf3ab17e7d9d3ece6de8a2eb2c496d7`、`linux/arm64` `f1745b06ef0d634a2d70551589704f844627abbfe90cbad90cd89690bdce427e`、`darwin/arm64` `27136cd7961b63c06df7793f60e4d574907d823a704d3b03251cb19bf9ec6f4b`、`windows/amd64` `38ed2b2a33796d0c582d5820fd33b14b2f807d5718e3aa5e81579f32bd3ce051`。

边界：15 分钟是服务运行期间的清理周期，机器关机期间不会独立执行；下次启动会立即补做。底层文件系统在删除阶段失败时仍可能留下部分到期记录，后续周期会重新扫描；完整认证失败则一项也不删。原生适配器采集和三平台真实原文端到端仍待完成。交叉编译不构成 macOS/Windows 原生运行证据。本批未提交、推送或合并。
