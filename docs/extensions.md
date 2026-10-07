# 本地 VSIX 与扩展宿主

`extensions.Install(root, path)` 安装本地 VSIX；`List(root)` 选择每个扩展的最新数字版本；`Start(ctx, workspace, installed)` 启动 Node.js 22+ 的独立宿主。扩展使用当前用户权限，宿主不是沙箱。安装器拒绝目录穿越、绝对路径、符号链接、Windows 保留名、大小写冲突和超限数据；限制为 4096 项、单文件 16 MiB、总解压量 64 MiB、manifest 256 KiB。

## 实现范围

| API | 当前能力 |
| --- | --- |
| 激活 | CommonJS main、activate/deactivate、subscriptions、命令惰性激活、onLanguage、onStartupFinished、* |
| commands | registerCommand、executeCommand、getCommands |
| 文档 | 持续存在的 TextDocument 对象、UTF-16 offsetAt/positionAt、行/范围/词读取、活动编辑器、未保存内容和版本 |
| 编辑 | TextEditor.edit、WorkspaceEdit、文档保存、单选区；由 Go UI 版本检查和事务预检后确认结果 |
| 事件 | 文档打开/修改/关闭/保存、活动编辑器、选区、配置和诊断 |
| 语言 | 补全、悬停、定义提供者注册和调用；DiagnosticCollection 发布、删除、清空 |
| window | 消息、输出频道、状态栏、showTextDocument；消息按钮选择尚未实现 |
| workspace | 单工作区、文档、fs 读取/写入/stat/readDirectory/createDirectory/delete；配置 get/has/inspect/update 的 JSON 文件实现 |
| 类型 | Uri、Position、Range、Selection、TextEdit、WorkspaceEdit、CompletionItem/List、Hover、MarkdownString、Location、Diagnostic、取消令牌、Disposable/EventEmitter |
| 持久状态 | 提供 storageRoot 后，globalState/workspaceState 跨宿主启动保存；按扩展和工作区隔离 |

候选原生桥接支持独立视图身份、`ViewColumn` 1–9/Active/Beside、隐藏打开、
`showTextDocument` 列/保留焦点/选区选项、`revealRange` 和可见/列/范围/选区事件。
客户端须提供 editorGroups/openDocument 能力，旧客户端的缺失选项明确报错。
精确协议、临时进程文件清理、实例/异步焦点保护与验证状态见
[扩展编辑器契约](../agent%20docs/extension-editors.md)。预览标签、tabGroups、
多光标/snippet、undo-stop 合并和完整配置/装饰仍未实现。

## Go 应用接入

先用 `Host.Register` 注册 `workspace/applyEdit`、`workspace/saveDocument`、`window/showTextDocument`、`window/setSelection`，处理器通过 `Context.Dispatch` 修改原生状态。初始化参数包含 `documents`、`active`、`storageRoot` 和 `clientCapabilities`；gocode 的 [接入代码](https://github.com/neko233-com/gocode/blob/main/extensions_ui.go) 提供完整示例。

保存处理器应后台写入冻结快照，完成后回到 UI 线程确认版本。返回当前 `document` 和 `saved`；仅匹配已写入版本时返回 `saved: true`。每次成功保存为文档提供递增的 `saveId`，并在保存通知与 RPC 响应中使用同一个值，扩展宿主据此只触发一次 `onDidSaveTextDocument`。失败或保存期间的新编辑只更新状态，不触发成功事件。

`Host.Call(ctx, "syncDocument", params, nil)` 同步版本、修改状态、选区和增量 changes。打开和显式恢复发送完整 text，普通修改只发送增量事务。`execute` 执行命令，`provideCompletionItems` / `provideHover` / `provideDefinition` 调用匹配提供者。Go 预检 WorkspaceEdit 的全部文档及版本后再修改；修改确认返回新的文档状态，扩展的 Promise 不会在实际编辑前伪报成功。

宿主复用框架 LSP 帧传输，支持双向 RPC 和异步并发调用。console 日志走 stderr。调用期限到达会终止整个宿主，以中断同步 JS 死循环。消息/输出等 Events 是有界通知，消费者需持续读取；`DroppedEvents()` 报告丢弃数量。原生编辑使用有响应 RPC，不走可丢弃的通知队列。

## 兼容缺口

这不是完整 [VS Code API](https://code.visualstudio.com/api/references/vscode-api) 或官方 [扩展宿主](https://code.visualstudio.com/api/advanced-topics/extension-host) 的实现。未知 API 明确抛错；尚缺调试、任务、终端、webview、TreeView、自定义编辑器、snippet/tabstop、多选区、语义 token、完整配置贡献/JSONC、文件监听、远程宿主、SecretStorage、Marketplace 等。提供者协议支持悬停/定义，但 gocode 尚未提供相应的完整交互。VSIX 依赖的第三方模块也必须包含在扩展中。

官方 Copilot VSIX 尚未通过兼容验收。gocode 当前使用官方 Language Server 和 Go SDK，和“运行完整官方 VSIX”分别验收。Hello Native 与真实 VSIX fixture 覆盖原生编辑、UTF-16/CRLF、版本更新、语言提供者和跨进程持久状态；样例通过不代表所有扩展都能运行。
