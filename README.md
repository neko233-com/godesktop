# godesktop

[![CI](https://github.com/neko233-com/godesktop/actions/workflows/ci.yml/badge.svg)](https://github.com/neko233-com/godesktop/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/neko233-com/godesktop.svg)](https://pkg.go.dev/github.com/neko233-com/godesktop)

用 **Go 1.27** 开发 Windows 和 macOS 原生桌面应用。以 [GPUI](https://github.com/zed-industries/zed/tree/main/crates/gpui) 的声明式视图、持久状态和 GPU 渲染思路为参考，目标是让 Go 开发者拥有可用于复杂桌面软件的工具链。

应用逻辑、状态和布局使用 Go；小型 C ABI 桥接系统窗口、文本引擎和图形 API。UI 不依赖 WebView、浏览器或 Rust。可选的 VSIX 扩展宿主使用独立 Node.js 进程，核心窗口不需要 Node.js。

**当前是实验性原型，API 尚未稳定。已实现原生窗口、布局和交互闭环；尚未达到 GPUI 的功能或性能成熟度。** 没有经过与 GPUI 的同场景性能对比，不承诺已经能替换完整的编辑器或生产应用。功能边界见 [路线图](docs/roadmap.md)。现代 GPU 改造和实际验证证据见 [绘制验收](docs/rendering-modernization.md)。Windows 硬件窗口使用 D3D12/DXIL、DXGI flip swapchain 和 DirectWrite；软件适配器仍由 D3D12 GPU 管线绘制，完成的 committed target 经 fence 确认后显示为原生 DIB。v0.17.0 两条已发布显示路径的实际验证范围见 [设备生命周期记录](agent%20docs/windows-device-removal.md)。

v0.4 增加版本化编辑缓冲区、原生选区/剪贴板接口、LSP 和 VSIX 编辑/语言提供者。gocode 接入官方 Copilot Language Server 与 Go SDK。API 见 [编辑与 LSP](docs/editor-and-lsp.md)，完整 VS Code 目标的当前覆盖见 [gocode 功能矩阵](https://github.com/neko233-com/gocode/blob/main/docs/vscode-parity.md)。

v0.10.0 将可选 VSIX 宿主接到真实原生编辑视图，支持共享文档、独立视图身份、可见编辑器/列/范围/选区事件和有序的原生打开、显示、选区、reveal。关闭重开与异步焦点回执保护旧引用；Node 的来源/配置文件有界并在退出时清理。已通过 Windows amd64、macOS Intel/ARM 和 Ubuntu 五个平台 CI，具体范围见 [扩展编辑器契约](agent%20docs/extension-editors.md)。

v0.12.0 已发布并通过五平台 CI，支持 Windows 与 Mac 的原生彩色字体。Windows 保留 DirectWrite 整形，彩色字形按物理基线相位缓存为原生预乘 RGBA，普通文字使用 R8；16 个字形纹理槽保持混合文字的一次绘制。实际颜色、透明度、独立原生参考图、缓存与恢复的验证范围见 [彩色字形记录](agent%20docs/color-glyphs.md)。gocode v0.19.0 已使用公开模块，通过独立五平台、正式发行字节与本机安装验收。

v0.13.0 将可选 VSIX Terminal API 接到真实 ConPTY/PTY，覆盖 PID、环境、Unicode
输入、显示/隐藏/焦点与生命周期；gocode v0.20.0 已完成五平台和本机验收。
gocode v0.21.0 增加真实 Git 状态、暂存/取消暂存、索引提交与原生左右差异，
已通过五平台、发行字节/回滚和本机安装验收。能力、内存边界与合并/扩展 SCM 等缺口见
[Git 工程记录](https://github.com/neko233-com/gocode/blob/main/agent%20docs/scm.md)。

gocode 的安装与更新通过独立的 [发布入口](https://github.com/neko233-com/gocode/releases)
提供 Windows x64 MSI／ZIP 和 macOS Intel／ARM 包，并维护免费 CLI、winget manifest、
Scoop bucket 和 Homebrew cask。原生更新页支持自动路由、GitHub 直连和手动镜像；
更新验证发布签名、哈希和程序来源后选择下次启动版本。验证证据与未实现功能见
[gocode 工程记录](https://github.com/neko233-com/gocode/blob/main/agent%20docs/status.md)。

gocode v0.13.0 增加原生流式全文搜索：未保存快照、Unicode 大小写／整词、Go 正则、
包含／排除和 Git 忽略、分组结果及核验后的 UTF-16／大文件跳转。Windows amd64
真实 1 GiB、Mac Intel／ARM 三种像素密度、发行字节与本机安装均已验收。
完整 VS Code 搜索兼容仍需实现，边界见
[搜索契约](https://github.com/neko233-com/gocode/blob/main/agent%20docs/search.md)。

gocode v0.14.0 使用公开 godesktop v0.8.0 的后台预备事务，增加原生工作区替换预览、
完整缓冲区提交、后台保存、过期内容／点击身份保护及可撤销的失败状态。五平台、
发行字节和本机安装已验收。保存按文件完成；完整 diff、逐项控制和工作区全局撤销
仍待实现，具体边界见 [替换契约](https://github.com/neko233-com/gocode/blob/main/agent%20docs/replace.md)。

## 平台

| 平台 | 窗口 | 图形 | 文本 | CI 架构 |
| --- | --- | --- | --- | --- |
| Windows x64（已验证 Windows 11、Server 2022/2025） | Win32 | Direct3D 12 / SM6，三帧 fence；硬件 DXGI、软件适配器 committed target + 原生 DIB | DirectWrite，普通 R8／彩色 RGBA 缓存 | amd64 / GOAMD64=v1 |
| macOS（已验收 15 Intel/ARM；13/14 新编译控制待验证） | AppKit | Metal，实例化合批、三帧异步环；14+ CAMetalDisplayLink | CoreText，普通 R8／彩色 RGBA 缓存 | arm64 / amd64 |

Go 1.27 的 macOS 最低版本是 13，见 [官方发布说明](https://go.dev/doc/go1.27)。macOS 必须具备 Metal 设备。v0.17.0 的局部浮点编译控制已在 Xcode16/macOS15 验收，尚未独立验证 macOS13/14 的旧编译器；Go 的最低系统版本不等于这项框架能力的完整支持证明。Linux 仅能构建和测试可移植核心；调用 `Run` 会明确返回不支持错误。

Windows 启动需要实际可查询的 `ID3D12Device5`，它用于有界失败收尾。接口文档列出 Windows 10 1809，而 [RemoveDevice 方法文档](https://learn.microsoft.com/en-us/windows/win32/api/d3d12/nf-d3d12-id3d12device5-removedevice) 的最低版本是 build 20348；不能仅凭接口存在声称所有旧 Windows 10 都完整支持。旧系统仍需独立验证。软件显示的 `d3d12-fence` 表示真实 GPU 完成，未宣称具有 DXGI 的显示同步。

## 开始使用

需要 Go 1.27+ 和 `CGO_ENABLED=1`。Go 1.27 已发布，见 [Go 官方公告](https://go.dev/blog/go1.27)。

Windows：安装带 `gcc` / `g++` 的 64 位 MinGW-w64（例如 WinLibs 或 MSYS2 UCRT64），并将其 `bin` 目录加入 `PATH`。仅安装 Visual Studio 的 `cl.exe` 不够。原生桥接静态链接 MinGW 支持库，使用系统 D3D12/DXGI/DirectWrite DLL；彩色字形缓存使用系统 Direct2D／D3D11 WARP。需要支持 Shader Model 6.0 的硬件或系统 WARP，运行时不需要 DXC。

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
go get github.com/neko233-com/godesktop@latest
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

`WindowOptions.CloseRequested` 在 UI 线程处理系统关闭、Alt+F4 和 macOS 退出菜单；
返回 `false` 可保留窗口并显示未保存文档的确认界面。自定义标题栏关闭按钮应调用
`cx.RequestClose()`。确认保存／丢弃后才调用 `cx.Quit()`；它是明确的强制退出路径，
也用于异常恢复。该 API 的 Windows 原生拒绝／确认、强制退出及关闭回调 panic
均有真实进程验证；macOS 在 CI 运行同一原生流程。

## 当前功能

- 原生单窗口和平台事件循环，高 DPI / Retina 坐标。
- `Row` / `Column` 的顺序布局、尺寸、内边距、间距和按权重分配剩余空间的 `Grow`。
- 文本、圆角背景、继承矩形裁剪、按钮、键盘焦点、指针捕获取消。
- `ClipRounded` 保留祖先圆角裁剪；当前候选的 `Shadow(ShadowStyle)` 在元素后方绘制原生 GPU 阴影，`Position(x, y)` 定位直接 `Stack` 子元素。阴影不扩展命中区域，继承父裁剪；公开版本和数值／实际 GPU 验收范围见 [阴影工程记录](agent%20docs/shadows.md)。
- 不可变 RGBA `Bitmap` / `Image`，D3D12/Metal 纹理复用、透明混合和宽高比裁剪；见 [图片 API](docs/native-images.md)。
- 按需重绘、UI 调度、批量传输命令、有界文本缓存。
- `Flex` 零基准空间分配、独立横纵内边距、平台字体选择、矢量图标、容器点击。
- `Viewport` 双轴视口裁切和偏移、非交互元素的 `Key` 几何查询、滚轮指针坐标和按键释放；见 [视口与输入](docs/viewports.md)。
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

以原生 GPU API 和事件驱动渲染为基础，逐步建设能承载大型桌面软件的框架。两种后端使用 80 字节 GPU 实例、着色器裁剪、相邻命令合批和三个独立上传缓冲；GPU 完成后才复用缓冲，正常绘制遇到在途资源时延后重绘。DirectWrite 与 CoreText 按字形缓存普通 R8 覆盖率与彩色预乘 RGBA。两种后端提供有界的 GPU 资源恢复；Windows 实际 RemoveDevice 和 Metal 真实提交后的诊断恢复已通过 CI。实际 eGPU 故障场景和大列表虚拟化仍需补充。

macOS 可运行 `CGO_ENABLED=1 go run ./internal/renderstress -require-backend metal`，检查真实 GPU 提交、三组缓冲复用、2048 个圆角矩形和 32 个共享文本命令的合批及上传量，并输出 CPU 帧编码 P50/P95。计数来自原生渲染器；没有 GPUI 同机对比，也不把 CI 虚拟环境中的数字当作真实设备性能保证。布局 benchmark 只测 Go 核心。

Windows 设置 `CGO_ENABLED=1` 后可运行 `go run ./internal/renderstress -require-backend direct3d12 -require-frame-clock dxgi -glyph-atlas`。该测试让文字每帧变化，检查字形复用、图集字节上限、实际 GPU 完成、空闲暂停与唤醒。`-glyph-eviction` 另行验证大字形触发缓存淘汰时的资源生命周期。

MIT License。GPUI 是设计参考，本项目未复制其实现代码。
