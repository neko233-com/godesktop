# godesktop

[![CI](https://github.com/neko233-com/godesktop/actions/workflows/ci.yml/badge.svg)](https://github.com/neko233-com/godesktop/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/neko233-com/godesktop.svg)](https://pkg.go.dev/github.com/neko233-com/godesktop)

用 **Go 1.27** 开发 Windows 和 macOS 原生桌面应用。以 [GPUI](https://github.com/zed-industries/zed/tree/main/crates/gpui) 的声明式视图、持久状态和 GPU 渲染思路为参考，目标是让 Go 开发者拥有可用于复杂桌面软件的工具链。

应用逻辑、状态和布局使用 Go；小型 C ABI 桥接系统窗口、文本引擎和图形 API。UI 不依赖 WebView、浏览器或 Rust。可选的 VSIX 扩展宿主使用独立 Node.js 进程，核心窗口不需要 Node.js。

**当前是实验性原型，API 尚未稳定。已实现原生窗口、布局和交互闭环；尚未达到 GPUI 的功能或性能成熟度。** 没有经过与 GPUI 的同场景性能对比，不承诺已经能替换完整的编辑器或生产应用。功能边界见 [路线图](docs/roadmap.md)。现代 GPU 改造和实际验证证据见 [绘制验收](docs/rendering-modernization.md)；Windows 默认窗口使用 D3D12/DXIL、DXGI flip swapchain 和 DirectWrite R8 字形图集。

## 平台

| 平台 | 窗口 | 图形 | 文本 | CI 架构 |
| --- | --- | --- | --- | --- |
| Windows 10+ | Win32 | Direct3D 12 / SM6，三帧 fence、DXGI 显示同步；无合适硬件时 WARP | DirectWrite，共享 R8 字形图集 | amd64 |
| macOS 13+ | AppKit | Metal，实例化合批、三帧异步环；14+ CAMetalDisplayLink | CoreText，R8 字形图集 | arm64 / amd64 |

Go 1.27 的 macOS 最低版本是 13，见 [官方发布说明](https://go.dev/doc/go1.27)。macOS 必须具备 Metal 设备。Linux 仅能构建和测试可移植核心；调用 `Run` 会明确返回不支持错误。

## 开始使用

需要 Go 1.27+ 和 `CGO_ENABLED=1`。Go 1.27 已发布，见 [Go 官方公告](https://go.dev/blog/go1.27)。

Windows：安装带 `gcc` / `g++` 的 64 位 MinGW-w64（例如 WinLibs 或 MSYS2 UCRT64），并将其 `bin` 目录加入 `PATH`。仅安装 Visual Studio 的 `cl.exe` 不够。原生桥接静态链接 MinGW 支持库，使用系统 D3D12/DXGI/DirectWrite DLL；需要支持 Shader Model 6.0 的硬件或系统 WARP，运行时不需要 DXC。

```powershell
git clone https://github.com/neko233-com/godesktop.git
cd godesktop
$env:CGO_ENABLED = '1'
go run ./examples/counter
```

macOS：安装 Xcode Command Line Tools，使用系统 Clang 和 SDK 编译 AppKit/Metal 桥接。

```sh
xcode-select --install
git clone https://github.com/neko233-com/godesktop.git
cd godesktop
CGO_ENABLED=1 go run ./examples/counter
```

在已有应用中添加：

```sh
go get github.com/neko233-com/godesktop
```

```go
package main

import (
    "fmt"
    "log"

    ui "github.com/neko233-com/godesktop"
)

func main() {
    count := 0
    err := ui.Run(ui.WindowOptions{
        Title: "Hello Go desktop", Width: 640, Height: 400,
    }, func(cx *ui.Context) *ui.Element {
        return ui.Column(
            ui.Text(fmt.Sprintf("Count: %d", count)).FontSize(32),
            ui.Button("Increase", func(*ui.Context) {
                count++
            }).Key("increase"),
        ).Padding(32).Gap(20)
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

从 `main` goroutine 调用 `Run`。视图和点击回调都在 UI 线程执行，点击后自动重绘。按钮支持鼠标点击、Tab / Shift+Tab 焦点遍历、Enter / Space 激活。用唯一 `Key` 保持按钮在树结构变化时的身份；省略时使用树路径。

后台工作通过 `Dispatch` 回到 UI 线程修改状态：

```go
go func() {
    result := loadData() // 网络或文件操作
    cx.Dispatch(func() {
        data = result // 仅在 UI 线程读写应用状态
    })
}()
```

`Dispatch` 线程安全并自动请求重绘；关闭后返回 `false`。`Invalidate` 和 `Quit` 也可以从后台调用。不要在 UI 回调中做阻塞 I/O。`Context` 不能跨越两次 `Run` 复用。

## 当前功能

- 原生单窗口和平台事件循环，高 DPI / Retina 坐标。
- `Row` / `Column` 的顺序布局、尺寸、内边距、间距和按权重分配剩余空间的 `Grow`。
- 文本、圆角背景、继承矩形裁剪、按钮、键盘焦点、指针捕获取消。
- 按需重绘、UI 调度、批量传输命令、有界文本缓存。
- `Flex` 零基准空间分配、独立横纵内边距、平台字体选择、矢量图标、容器点击。
- 自定义标题栏拖动和窗口按钮；Unicode 字符、按键和滚轮输入回调。
- 本地 VSIX 安装和 Node 扩展宿主，支持范围见 [扩展 API](docs/extensions.md)。
- 无第三方 Go 模块依赖；不启用 cgo 时核心仍可测试，窗口启动返回明确错误。

布局是基础线性布局：溢出裁剪，不实现完整 CSS flexbox、自动换行、滚动、最小/最大尺寸或自动缩小。尺寸为设备无关像素。视图每次重建元素树，应用状态由调用方持有；还没有 GPUI 的 Entity 系统。

## gocode 子仓库

[gocode](https://github.com/neko233-com/gocode) 是独立公开 Git 仓库，以 `godesktop/gocode` submodule 保存固定提交。它用本框架实现 VS Code Dark Modern 风格的原生工作区、文件标签、基础编辑/保存、扩展面板和命令执行。不是完整 VS Code，也没有宣称任意扩展兼容或像素完全一致。

```sh
git submodule update --init gocode
cd gocode
# Windows PowerShell: $env:CGO_ENABLED='1'
CGO_ENABLED=1 go run . -workspace .
```

`gocode` 可单独 clone，其 `go.mod` 固定依赖已发布的本框架版本。开发两个仓库时，可在父目录用 `go work init . ./gocode` 建立本地工作区；不要提交 `go.work`。更新 submodule 前先在子仓库提交并推送，再在父仓库提交新的 gitlink。

## 验证与构建

```sh
CGO_ENABLED=1 go test -race ./...
CGO_ENABLED=1 go vet ./...
CGO_ENABLED=1 go run ./examples/counter -smoke
go test -run '^$' -bench . -benchmem ./...
```

PowerShell 中先设置 `$env:CGO_ENABLED = '1'`，再运行上述命令（去掉命令前的环境变量赋值）。原生 smoke 会创建真实窗口，绘制两帧、检查调度后的状态更新并退出。需要可用的桌面会话和图形设备；它不校验所有像素、字体效果或键鼠行为。

```powershell
go build -trimpath -ldflags='-s -w -H=windowsgui' -o bin/counter.exe ./examples/counter
```

```sh
CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o bin/counter ./examples/counter
```

Windows 的架构目标为 **x86-64 / amd64**。完整自动化检查可以运行 `powershell -File scripts/test-windows-amd64.ps1`：三轮 race、严格 cgo、真实 HWND 消息与像素、重复启动、fuzz、覆盖率门槛以及普通/GUI EXE 的 64 位架构和依赖检查。范围与边界见 [自动化验证](docs/testing.md)。

CI 在 Windows Server 2022 / 2025 上运行完整 amd64 检查，macOS arm64 / amd64 执行原生编译和 smoke，Linux 执行核心与无 cgo 测试。计数器程序和覆盖率报告可以从 [Actions artifacts](https://github.com/neko233-com/godesktop/actions) 下载。macOS 的 `.app` 打包、签名和公证尚未实现；cgo 原生后端需要各平台的工具链，不能只设置 `GOOS` 从 Windows 直接交叉编译 macOS。

## 设计与性能

[架构](docs/architecture.md) · [现代绘制验收](docs/rendering-modernization.md) · [路线图](docs/roadmap.md) · [贡献说明](CONTRIBUTING.md)

以原生 GPU API 和事件驱动渲染为基础，逐步建设能承载大型桌面软件的框架。两种后端使用 80 字节 GPU 实例、着色器裁剪、相邻命令合批和三个独立上传缓冲；GPU 完成后才复用缓冲，正常绘制遇到在途资源时延后重绘。DirectWrite 与 CoreText 均按字形缓存 R8 覆盖率。Windows 已实现设备丢失后的资源重建；Metal 设备移除恢复和大列表虚拟化仍需实现。

macOS 可运行 `CGO_ENABLED=1 go run ./internal/renderstress -require-backend metal`，检查真实 GPU 提交、三组缓冲复用、2048 个圆角矩形和 32 个共享文本命令的合批及上传量，并输出 CPU 帧编码 P50/P95。计数来自原生渲染器；没有 GPUI 同机对比，也不把 CI 虚拟环境中的数字当作真实设备性能保证。布局 benchmark 只测 Go 核心。

Windows 设置 `CGO_ENABLED=1` 后可运行 `go run ./internal/renderstress -require-backend direct3d12 -require-frame-clock dxgi -glyph-atlas`。该测试让文字每帧变化，检查字形复用、图集字节上限、实际 GPU 完成、空闲暂停与唤醒。`-glyph-eviction` 另行验证大字形触发缓存淘汰时的资源生命周期。

MIT License。GPUI 是设计参考，本项目未复制其实现代码。
