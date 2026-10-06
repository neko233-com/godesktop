# 版本化编辑与 LSP

`editor.Buffer` 是 UI 线程持有的文本模型。`Position.Character`、`OffsetAt` 和增量变更的长度使用 UTF-16 单元，以便和 VS Code/LSP 交换坐标；Go 字符串仍然使用 UTF-8。拒绝拆开代理对、重叠事务、越界范围、无效 UTF-8 和 NUL。

```go
buffer, _ := editor.New("你😀\r\n")
change, err := buffer.Apply([]editor.Edit{{
    Range: editor.Range{
        Start: editor.Position{Line: 0, Character: 1},
        End: editor.Position{Line: 0, Character: 3},
    },
    Text: "界",
}}, nil)
// change.Version == 2; buffer.Text() == "你界\r\n"
```

事务先验证所有范围，再从后向前应用。撤销/重做恢复内容和选区，并继续增加文档版本；保存点通过独立修订标识判断。历史记录限制为 2048 项、16 MiB 文本；单次超大事务仍可应用，但清空撤销历史。文档统一使用检测到的 LF 或 CRLF，混合行尾会规范化。

`Snapshot()` 的内容不可变，可以传给后台补全、搜索和语言服务。不要从后台线程读取或修改 Buffer。gocode 将路径、版本、光标和请求序号一起带到后台；响应返回 UI 后再次核对，过期建议不会修改新的内容。

## 原生编辑支持

`Stack` 按绘制顺序叠加元素，光标和选区不会挤开文本。`MeasureText` 使用平台整形引擎测量文本；`Context.ElementBounds(key)` 返回上一轮有效布局中交互元素的 DIP 边界。输入携带修饰键和独立 Repeat 标记；重复按键用于编辑，普通按钮不会因长按 Enter 连续执行。

`TextAdvance(text, size, font)` 返回不含布局留白或像素取整的原生文字步进，适合终端等等宽网格和光标定位。macOS `MeasureText` 包含标签留白，不能用它的单字符宽度乘列数。两者均在 UI 线程调用，字体/字号应与绘制一致；没有原生后端时仅使用模型估算值。

`ReadClipboard` / `WriteClipboard` 在 UI 线程调用，Windows 使用 CF_UNICODETEXT，macOS 使用 NSPasteboard。Windows 剪贴板往返测试仅在设置 `GODESKTOP_CLIPBOARD_TEST=1` 的一次性 CI runner 上执行。完整 IME、字素簇导航、双向文本点击、多光标和水平滚动仍需补齐。

## LSP

`lsp.Start(ctx, lsp.Command{...})` 启动隐藏的独立服务进程，`Call` / `Notify` 使用 JSON-RPC 2.0 的 Content-Length 帧。`Register` 处理服务端请求，`Notifications` 接收异步消息。上限为单帧 16 MiB、头部 8 KiB、64 个并发服务端处理器；通知队列有界，`DroppedNotifications()` 报告丢弃数量。

请求取消发送 `$/cancelRequest`，正常响应的服务可继续使用；阻塞写入会关闭传输，防止长期占用 goroutine。`Close` 关闭管道、结束进程并等待回收。用生命周期 Context 启动进程，对初始化和单次请求另设期限。核心 LSP 包不依赖 Copilot 或第三方 Go 模块。

gocode 的官方 Copilot 接入在独立仓库中：版本化 didOpen/didChange/didClose/didFocus、inlineCompletion、显示通知和实际接受后的 executeCommand。聊天使用官方 Go SDK。[GitHub 官方 LSP 文档](https://github.com/github/copilot-language-server-release)、[官方 Go SDK](https://github.com/github/copilot-sdk/tree/main/go) 是协议依据。

测试包括 UTF-16/CRLF、不可变快照、事务原子性、保存点、随机编辑/撤销 oracle、碎片帧、乱序并发响应、双向请求和阻塞/取消。真实 VSIX 和 gocode 原生验收覆盖从 Node 扩展到 Go 编辑事务的完整路径。
