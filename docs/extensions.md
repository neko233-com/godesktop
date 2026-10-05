# 本地 VSIX 与扩展宿主

`extensions.Install(root, path)` 安装本地 VSIX；`extensions.List(root)` 为每个扩展选择最新的数字版本；`extensions.Start(ctx, workspace, installed)` 启动独立 Node.js 宿主。需要 Node.js 22+，CI 使用 Node.js 24。扩展以当前用户权限运行，应只安装可信扩展。

安装器读取 `extension/` 目录，拒绝目录穿越、绝对路径、符号链接、Windows 保留名、大小写冲突、重复文件及超限数据。限制为 4096 项、每文件 16 MiB、总解压量 64 MiB、manifest 256 KiB。版本要求数字 `major.minor.patch`；已安装版本不可覆盖。

## 当前 API

| VS Code API | 实现范围 |
| --- | --- |
| `commands` | registerCommand、executeCommand、getCommands；manifest 命令自动惰性激活 |
| 激活 | CommonJS `main`、activate/context/subscriptions、`onCommand`、`*`、onStartupFinished |
| `window` | 信息/警告/错误消息事件、输出频道事件、状态栏事件、showTextDocument 打开事件 |
| `workspace` | 单工作区 folders/rootPath、读取 UTF-8 文档、只读 fs readFile/stat；configuration 仅返回默认值 |
| 基本类型 | Disposable、EventEmitter、Uri、Position、Range、Selection |
| 扩展状态 | workspaceState/globalState 是宿主进程内 Memento，尚不跨启动持久化 |

调用 `Host.Call(ctx, "initialize", nil, &commands)` 完成启动激活；`Host.Call(ctx, "execute", map[string]any{"command": id, "args": args}, &result)` 执行命令。Go 应用持续消费 `Host.Events` 并通过 `Context.Dispatch` 更新 UI。协议使用请求 ID，扩展 console 日志走 stderr，不混入 JSON 协议。宿主处理异步并发请求；调用超时会终止整个宿主以中断同步死循环。事件缓冲为 256 条，消费者过慢时后续事件丢弃。

## 不兼容范围

没有承诺完整兼容 [VS Code API](https://code.visualstudio.com/api/references/vscode-api) 或官方 [扩展宿主](https://code.visualstudio.com/api/advanced-topics/extension-host)。未知 API 明确抛错，不静默伪装成功。语言服务、LSP、调试器、终端、webview、TreeView、自定义编辑器、Web Worker 扩展、Marketplace、主题/语法贡献、远程工作区及需要完整 VS Code 工作台的扩展尚不支持。`showTextDocument` 返回基础 document 描述，编辑器选区/编辑 API 未实现；消息没有按钮选择返回值。

`gocode` 自带的 Hello Native 是实际 `require('vscode')` 扩展，通过标准 VSIX 安装路径加载，验证命令、消息、输出、状态栏和 README 打开。该样例通过不代表全部现有扩展都能运行。
