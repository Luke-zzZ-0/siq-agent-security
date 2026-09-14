# WorkBuddy Windows 下一批准备：安装源码只读调查

日期：2026-09-14。关联 SIQ 候选：`ebc472f2e46aa7de837afe9d6a0ed422eef51cd0`。

结论：已确认本机 WorkBuddy 桌面安装包存在配置目录重定向入口、桌面向内置 Agent CLI 传递插件设置的源码链，以及该内置组件的 PreToolUse veto 合同。**尚未验证 WorkBuddy 桌面会话的账号隔离、工具工作目录隔离或实际前置拒绝。当前 WorkBuddy matrix 全部保持 `not_run`；本报告仅供下一批准备，不能提升为 `native_desktop` 验收证据。**

本次只读取 `C:\Program Files\WorkBuddy\resources` 中安装文件和官方文档；未读取用户账号、凭据、历史或设置文件，未启动 WorkBuddy、内置 CLI、GUI、模型、Host 任务或插件，也未改适配器、注册插件或变更设置。没有用 CodeBuddy CLI 测试替代 WorkBuddy 桌面测试。

## 安装身份与可重复定位

安装文件是 `resources/app.asar` 和 `resources/app.asar.unpacked/`，前者大小 297844411 字节，共 20474 个索引条目。`app.asar/package.json` 声明 `@genie/workbuddy-desktop`、产品 WorkBuddy、版本 `5.5.6`、入口 `main/index.js`。这是一份独立 Electron 桌面安装包；不能由历史 VS Code 路径推断当前应用仍是 VS Code 外壳。

内置 `cli/package.json` 声明 `@genie/agent-cli`，其 `codebuddy`/`codebuddy-code` 等 bin 属于随桌面打包的运行组件。内置 `cli/product.json` 声明 WorkBuddy、目录 `.workbuddy`、版本 5.5.6、构建时间 `2026-09-10T10:36:20.308Z`、commit `5f9692923c93033111c51ad7b003eb80204a9b75`。后者是安装元数据，未独立核验为公开 Git 源码身份。

| 安装内相对路径 | SHA-256 |
| --- | --- |
| `app.asar/package.json` | `6f74f42f656d2156948bf7be945d0268e140b6d9f7b55645a7060717fc2e0c8c` |
| `app.asar.unpacked/cli/package.json` | `33697b29c19b73658a6986d813f469b59f9f0a31f1f08dfbd5ccf7cccf8cd819` |
| `app.asar.unpacked/cli/product.json` | `8f0822e68372ed1046e499f06c944cbd55ec170e49511d3bd478808593b58047` |
| `app.asar/main/app-instance.js` | `34cbbd4dd2c6cb00b82453a7cc6fc902e3b0caddf2905342e1e5c1cbc7047158` |
| `app.asar/main/legacy-auth-session-migrator.js` | `7eefe09ad385205e260a6b7cac480dd8fc4266d010d145d8795a3f1d5c0d6a1f` |
| `app.asar/main/node.js` | `ae1dd6eccf31d0572fcda9814e96150e8af415476140e2e540412eb31b1b0d90` |
| `app.asar/main/tar.js` | `0edf1e1a2fea2f9dd92119255d790af63d9759c270bdd10fdda2ea3378a3307e` |
| `app.asar.unpacked/cli/dist/codebuddy.js` | `eb018e35d80673db02ebdaa7547b72f43d130d660c9c907c338d35e2f7b42ff7` |

完整 17 个安装模块/文档的 SHA-256、大小、定位字符串、行号与字符偏移保存在归档的 `evidence/workbuddy-source-identity.json`。行号按解码后的原始文件 LF 计数，从 1 开始；字符偏移不是字节偏移。压缩 CLI 的单行很长，应结合 hash 和偏移定位。

归档中的 `evidence/workbuddy-source-identity.json` 保留本次原始身份报告的原字节。`evidence/workbuddy-source-identity.py` 是本报告自编读取器的派生副本，仅封装安装路径和输出参数：`--resources` 默认 `%ProgramFiles%/WorkBuddy/resources`，`--out` 必填，使用 exclusive 新建方式拒绝覆盖已有输出。参数解析位于安装文件读取之前，`--help` 直接退出。

复核命令（先 `cd` 到本归档根目录，在 PowerShell 使用已选定的 Python 3 环境）：

```powershell
python .\evidence\workbuddy-source-identity.py --help
New-Item -ItemType Directory -Force .\.tmp | Out-Null
python .\evidence\workbuddy-source-identity.py --out .\.tmp\workbuddy-source-identity-replay.json
```

自定义安装位置时增加 `--resources 'D:\WorkBuddy\resources'`。每次回放须使用新的 `.tmp` 输出文件名；若文件已存在，读取器会拒绝覆盖。回放输出不能写到归档原始 identity 路径。第二个 Python 命令是下一次只读回放说明，本次封装没有执行，也没有重新扫描安装。

原始身份读取器当时执行退出码为 0。它只解析 ASAR 索引并读取模块字节、独立文档与元数据；不 import、eval 或调用安装源码。归档没有收录闭源源码片段导出器。当前仅对派生读取器做 `py_compile` 和 `--help` 的轻量验证；其新路径参数未用于重扫安装。报告包含官方安装路径和公开配置键，无用户目录值或凭据。

## 配置隔离入口及尚未满足的边界

| 问题 | 只读证据与判断 |
| --- | --- |
| 桌面 `--user-data-dir` | 扫描 167 个 packed `main/*.js` 后，仅 `main/src.js:642,645` 的 Electron 日志辅助代码读取该参数；未发现桌面启动解析器将其作为受支持隔离合同。应用随后自行设置 userData，因此不能照搬通用 Electron/VS Code 参数并声称有效。 |
| `--extensions-dir`、`--profile`、`--workspace` | 同一范围未找到这些独立桌面参数；`--workspace-folder` 仅出现于 `main/e2b-filesystem.js:17862,17868` 的 devcontainer 命令。字符串扫描仅限定于上述文件范围，不是全产品不存在此能力的证明。官方 WorkBuddy 页面也未给出已核验的桌面隔离参数合同。 |
| 配置根环境入口 | `main/workbuddy-product-config.js:443` 的 `resolveWorkbuddyConfigDir()` 优先使用 `WORKBUDDY_CONFIG_DIR`，其次 `CODEBUDDY_CONFIG_DIR`，否则回落用户 home 下产品目录。`main/app-instance.js:81` 将选定 configDir 赋给内置组件的 `CODEBUDDY_CONFIG_DIR`，并同步 WorkBuddy 环境变量。 |
| Electron 数据目录 | `main/workbuddy-paths.js:84–96` 读取 `WORKBUDDY_USER_DATA_DIR`，否则用 configDir 下的 `app`，session 位于该 userData 下；`main/app-instance.js:99–100` 调用 Electron `setPath` 设置 userData/sessionData。此处是安装实现入口，不是已验证的公开 CLI 参数。 |
| 旧账号导入 | `main/legacy-auth-session-migrator.js:11–28` 在 Windows 独立通过 `APPDATA` 或 home/AppData/Roaming 定位 `WorkBuddy/User/globalStorage/state.vscdb`；第 51 行使用 Electron safeStorage 解密旧登录内容。仅重定向 WorkBuddy 配置根，不能据此证明不会探测/迁入旧账号。此调查未读取这个数据库。 |
| 旧历史与 OAuth 导入 | `main/launch-args.js` 的 `getLegacyAppDataDir` 系列函数（如 6181 行）及迁移代码（5504、6919、6926 行）使用旧平台目录和迁移标记；6885 行另读 `WORKBUDDY_USER_DATA_DIR`。必须核对这些源路径与当前测试账号的关系，不能只核对目标 configDir。 |
| 运行组件实际文件权限 | `main/node.js:1968–1969` 将客户端 ACP 文件 RPC 能力标为 false；附近源码说明 worker 直接进行 OS 文件操作。workspace/cwd 选择不等于 OS 沙箱，配置目录重定向也不限制工具能读写的全部路径。 |

源码还存在单实例锁和 Windows 启动修复预检。当前没有用 `WorkBuddy.exe --help` 试探未知参数，因为尚未确认该入口只展示帮助而不进入正常启动流程。这是本批只读边界，不是称程序不支持帮助。

下一批不能直接把 `WORKBUDDY_CONFIG_DIR` 与 `WORKBUDDY_USER_DATA_DIR` 两项设到私有目录，就宣布完成账号、历史、插件凭据、全部工具文件范围隔离。需要可核验的支持合同或独立的干净测试 Windows 用户/测试环境，并用合成旧目录和无秘密标记验证回落/迁移行为；这些都尚未执行。

## Hook 与 manifest：属于桌面内置运行组件的合同

官方 [WorkBuddy 插件系统](https://www.codebuddy.cn/docs/workbuddy/Plugins) 将 Hooks 列为插件组件，并描述桌面管理插件的路径。这支持桌面存在插件能力，尚不足以证明某次桌面工具调用已执行 SIQ 的前置 veto。

安装源码的连接关系如下：

1. `main/tar.js:1798,1868` 的插件读取支持 `.codebuddy-plugin` 与 `.workbuddy-plugin` 的 `plugin.json`；`mergeEnabledPlugins`（35588 行）把插件选择合入会话设置。
2. 同模块的会话插件合成区把 `sessionSettings.enabledPlugins` 交给 Agent CLI 设置；`main/node.js:7396` 把 `settingsPayload` 以 `--settings` 参数转交 worker。`main/sidecar-manager.js:2041` 的内置 CLI 路径解析明确涉及 `cli/dist/codebuddy.js`。
3. `app.asar.unpacked/cli/dist/codebuddy.js` 第 398 行含 PreToolUse 决策聚合中将 `permissionDecision=deny` 保留为 deny 的实际代码，第 885 行包含插件 `hooks/hooks.json` 及扩展插件 Hook 开关处理。这里只读取和定位代码，没有运行它。

因此 veto 合同确实随 WorkBuddy 安装提供，且存在桌面转交设置的静态链路；仍不能由此推断 SIQ 插件已加载、对应工具名 matcher 已命中、桌面消费了拒绝结果，或真实工具副作用已被阻断。单独启动此内置 CodeBuddy CLI，最多产生组件/CLI 层证据，不能替代完整 `native_desktop` 验收。

随安装提供的 `cli/dist/web-ui/docs/cn/cli/plugins-reference.md:288,327–360` 说明 manifest 元数据位置、必填 name 和 hooks 路径/数组/内联对象形式，默认 Hook 文件为 `hooks/hooks.json`。目录别名的文档是内置 CLI 组件参考；桌面源码直接确认了其中 `.codebuddy-plugin`/`.workbuddy-plugin` 两个读取入口。未找到一个可独立提取、带 WorkBuddy 桌面版本身份的 JSON Schema 文件，不能把 CLI 文档章节冒充新的桌面合同。官方在线参考对应 [CLI 插件参考](https://www.codebuddy.cn/docs/cli/plugins-reference)。

安装的真实内置插件 `resources/plugins/workbuddy-builtin/builtin-plugins/sheetagent/.codebuddy-plugin/plugin.json:18` 指向 `./hooks/hooks.json`；其 Hook 文件配置了 SubagentStop，并未配置 PreToolUse。这证明安装包使用此文件结构，不能作为前置 veto 样例已通过。

内置 `cli/dist/web-ui/docs/cn/cli/hooks.md` 的关键语义是：

- 594–608 行：退出码 2 对 PreToolUse 阻止工具调用；其他非零退出码属于非阻塞错误。**退出码 1 不能作为 SIQ 断服时的有效阻断证据。**
- 641–660 行：JSON 输出可使用 `hookSpecificOutput.hookEventName=PreToolUse` 及 `permissionDecision=deny`；`permissionDecisionReason` 用于原因。`ask` 是请求用户确认，不能当作拒绝；`allow` 有绕过权限检查的语义，不能无条件用作 SIQ 正常路径输出。
- 后置 Hook 发生在工具已运行后，不能承担本任务的前置阻断要求。官方在线组件参考：[CLI Hooks](https://www.codebuddy.cn/docs/cli/hooks)。

另一个官方页面 [腾讯云 Hook 系统](https://cloud.tencent.com/document/product/1831/137032) 虽使用 WorkBuddy Enterprise 产品标题，正文和导航定位 CodeBuddy Agent SDK；其 callback `decision: block` API 不能直接替换本安装命令 Hook 的 JSON/退出码合同，更不能据标题认定为已验证的 WorkBuddy 桌面 SDK 接口。

## 下一批需要完成的真实链路

1. 固定 WorkBuddy 安装版本与上述模块 hash，再取得/验证可独立使用的测试账号与测试目录。验证配置根、Electron 数据根、旧登录/历史迁移源、内置 worker 会话和工具 cwd 的实际去向；只使用合成配置与测试文件，不探测真实账号数据库。
2. 在该明确范围内通过受支持的 WorkBuddy 桌面插件入口安装测试适配器，记录 manifest、Hook 路径、版本、工具 matcher 和真实桌面进程/会话身份。配置变更先备份并有可卸载步骤。不得用 CLI 单独运行或源码元数据替代桌面 discovery、安装和绑定证据。
3. 用无网络、无生产影响的合成工具目标验证正常路径，再对越权、SIQ 决策服务不可达、返回损坏、适配器异常和超时逐项做负向测试。断服路径必须输出有效 deny 或明确退出 2，并观察实际工具未执行；若宿主超时/进程失败继续执行，应记录失败，不能把错误日志或 exit 1 算作拒绝。
4. 记录同一 SIQ candidate/binary、WorkBuddy 版本、插件身份、真实工具事件、决策、回执与前后文件状态。对回收/卸载和恢复做验证；当前没有任何一项此类原生桌面运行证据。

本报告不提出当前 adapter 改动，也不改变本批 matrix。Hook 语法、插件存在、安装源码环境变量与元数据盘点均属于下一批依据；原生 Windows WorkBuddy 的全部验收检查仍缺口。
