# GPU 绘制目标与验收

用户目标是使用现代原生绘制技术，并参考 Zed/GPUI 的架构。Windows 默认窗口采用 Direct3D 12/DirectWrite，macOS 采用 Metal/CoreText；两者均已接入 R8 字形图集、GPU 完成后复用的三帧资源环及显示同步调度。Metal 设备移除恢复、进一步 GPU 正确性/性能验收及 gocode 发布依赖仍需完成。

## 参考与技术选择

Zed 的 [Windows 报告](https://zed.dev/blog/windows-progress-report) 说明了 DirectX 11 自定义 HLSL、DirectWrite 字形光栅化、字形图集和显存问题。[Metal 优化报告](https://zed.dev/blog/120fps) 解释了多帧资源池和异步 CPU/GPU 工作。本项目参考这些设计，独立实现 Go 布局/C ABI 与原生 GPU 后端。

- Windows：Direct3D 12 显式队列、资源状态、fence、flip model swapchain，自定义 DXIL 着色器；DirectWrite 负责字体与字形。
- macOS：Metal 实例化绘制、相邻批次、三个独立上传缓冲、完成回调；CoreText 负责字体与字形。
- macOS 14+ 使用 [CAMetalDisplayLink](https://developer.apple.com/documentation/quartzcore/cametaldisplaylink) 提供的 drawable 和显示回调；合并重绘请求，空闲后暂停，Dispatch/输入重新唤醒。macOS 13 使用 MTKView 的显示同步循环，诊断明确报告该兼容路径。
- 共享：80 字节实例 ABI、GPU 顶点生成、片元裁剪、圆角/线段距离函数、预乘 alpha 与画家顺序。

着色器 API 的版本号不会单独证明性能。运行路径、异步资源所有权、字形复用、内存上限和真实 GPU 帧验证都必须有证据。

## 当前证据和剩余工作

| 项目 | 当前状态 | 必需证据 |
| --- | --- | --- |
| Metal 实例化/合批 | 已接入默认路径 | 两种 Mac CI 的 2048 矩形 + 32 文本压力场景，draw calls <= 2，上传量 = 实例数 × 80 |
| Metal 异步三帧资源环 | 已接入默认路径 | 三槽都被使用，最多三个在途帧，正常绘制显式 buffer waits = 0，退出时 submitted = completed |
| Metal 显示同步调度 | 已接入 macOS 14+ 默认路径，两种 Mac CI 已通过 | CAMetalDisplayLink 直接提供 drawable；连续请求合并；空闲期间 GPU 提交和回调计数不增加；Dispatch 能重新唤醒 |
| GPU 诊断 | 已实现原生计数与 CPU/GPU 时间 | 原生压力报告；不能以 Go view 次数代替 GPU 完成数 |
| Direct3D 12 着色器 | 可复现编译；默认窗口与离屏共用设备/PSO 创建 | RTX 5070 Ti 与 WARP 已通过 90 帧 × 16 个离屏 GPU 像素检查；[此前 Windows 2022/2025 离屏 CI 均通过](https://github.com/neko233-com/godesktop/actions/runs/37362077757) |
| Windows 默认 Direct3D 12 | 已接入 HWND flip swapchain、DXGI 显示同步、三帧 fence | 本机硬件已通过完整三轮 race/原生测试与 92 帧压力；实际 swapchain GPU 读回覆盖输入、缩放、最大化、恢复和重复启动 |
| Windows R8 字形图集 | 已接入默认 DirectWrite 文本路径 | 变化的文本 92 帧只光栅化 14 字形、29426 次命中、1 MiB 图集；中文/组合音标/emoji/阿拉伯文/裁剪的实际 GPU 像素；大字号淘汰压力和字节峰值 |
| Metal 共享字形图集 | 已接入默认路径；[8086828 两种 Mac CI 已通过](https://github.com/neko233-com/godesktop/actions/runs/37374018496) | 92 帧、14 次光栅化、29426 次命中、1 MiB 图集；4 次淘汰、16 MiB 峰值；实际 drawable 几何/Unicode 与独立 CoreText 对照 |
| Windows 设备丢失恢复 | 已实现；本机硬件/WARP 的实际 RemoveDevice 已通过，CI 待验收 | 同一窗口自动重建、至少 90 个后续完成帧、丢弃帧单独计数、正确 GPU 像素、三槽/空闲/字形图集回归 |
| Metal 设备移除恢复 | 尚未实现 | 设备选择、队列/帧槽/图集重建与有界恢复，旧完成回调隔离 |
| 绘制正确性与性能 | 部分验证 | 裁剪、透明混合、圆角、线段、文本及多帧读回；真实设备 P50/P95 与输入延迟 |
| gocode 消费新后端 | 仍依赖 v0.2.2 | 新框架版本及子模块更新后，在 Windows/两种 Mac 架构上重新验证 |

## 预编译 Windows 着色器

HLSL 为 `internal/platform/shaders/ui.hlsl`，实例 ABI 为 `gpu_scene.h`。DXIL 使用微软 DXC 1.9.2609.5，以 Shader Model 6.0 作为现代 Direct3D 12 的兼容基线；不要求应用运行时存在编译器。生成头文件记录两份源码的 SHA256。

```powershell
# 验证；缺少编译器时下载固定版本并检查官方发布文件的 SHA256
powershell -File scripts/validate-shaders.ps1
# 修改 HLSL / GPU ABI 后重新生成，并审阅二进制差异
powershell -File scripts/validate-shaders.ps1 -Regenerate
```

Metal 的渲染压力验证：

```sh
CGO_ENABLED=1 go run ./internal/renderstress -require-backend metal -require-frame-clock cametaldisplaylink -glyph-atlas -output metal-render-stress.json
CGO_ENABLED=1 go run ./internal/renderstress -require-backend metal -require-frame-clock cametaldisplaylink -glyph-eviction -output metal-glyph-eviction.json
CGO_ENABLED=1 go run ./internal/metaltest -output metal-pixels
```

Direct3D 12 设备和像素验收：

```powershell
$env:CGO_ENABLED = '1'
go run ./internal/dx12test -require-hardware -output .cache/dx12-hardware
go run ./internal/dx12test -warp -output .cache/dx12-warp
go run ./internal/dx12test -frames 7 -debug=false -output .cache/dx12-uninstrumented
```

该验证实际运行仓库中的预编译 DXIL，并在 fence 完成后从 GPU render target 复制 BGRA 像素。每帧检查实例步长、变化的颜色、画家顺序、透明混合、裁剪、圆角及抗锯齿、水平/对角线、R8 覆盖率纹理。第一个三帧批次通过独立 queue gate 延迟 GPU 执行，断言第四次尝试不能覆盖任何在途 allocator/上传缓冲，再释放 gate 并验证三帧各自的颜色。readback 的等待属于诊断过程；正常 submit 遇到未完成 fence 时只返回 defer。

默认设备优先选择支持 SM6 的硬件，缺少硬件时可使用 WARP；离屏报告明确记录 software、adapter 和 debug/GPU validation 是否可用。`-require-hardware` 拒绝 WARP，`-warp` 显式验证软件设备，`-require-debug-layer` 强制要求调试层。PNG 只包含离屏 GPU 验证场景；该夹具使用合成 R8 覆盖率，窗口字形图集由下述独立场景验证，gocode 的发布依赖尚未升级。

设备报告同时保存 vendor/device ID 和原始 DXGI flags。[微软 DXGI 文档](https://learn.microsoft.com/en-us/windows/win32/direct3ddxgi/d3d10-graphics-programming-guide-dxgi) 指出，主 Basic Render 适配器可能不设置 SOFTWARE 标志；`0x1414:0x008c` 仍按软件适配器识别并从硬件候选中排除。GitHub Windows runner 的 Basic Render/WARP 结果不能当作物理 GPU 性能数据；本机硬件证据来自明确要求硬件的 RTX 5070 Ti 验收。

7 帧验收关闭调试层，另外覆盖不启用 GPU 校验插桩时的同一着色器管线，以及三帧批次后剩余一帧的读回路径。默认和 WARP 的 90 帧验收继续覆盖帧资源的多轮复用。

## 默认 Windows 窗口与字体

```powershell
$env:CGO_ENABLED = '1'
$env:GODESKTOP_GPU_DEBUG = '1'
# 本机验收要求物理硬件；CI 默认路径允许明确的 WARP 回退
$env:GODESKTOP_GPU_ADAPTER = 'hardware'
go run ./internal/renderstress -require-backend direct3d12 -require-frame-clock dxgi -glyph-atlas -output .cache/dx12-window-glyph-stress.json
go run ./internal/renderstress -require-backend direct3d12 -require-frame-clock dxgi -glyph-eviction -output .cache/dx12-window-eviction.json
go run ./internal/renderstress -require-backend direct3d12 -require-frame-clock dxgi -device-recovery -output .cache/dx12-window-recovery.json
$env:GODESKTOP_GPU_ADAPTER = 'warp'
go run ./internal/renderstress -require-backend direct3d12 -require-frame-clock dxgi -glyph-atlas -output .cache/dx12-window-warp.json
go test -race -run '^TestWindowsAMD64NativeIntegration$' -count=1 .
```

所有场景走同一 HWND/D3D12 管线。DXGI frame-latency 对象与消息循环协同调度，显示就绪且该槽 fence 完成后才提交；正常帧不等待缓冲，缩放/退出才有界排空。三个帧槽各自持有 allocator、实例上传、描述符和图集上传暂存。

设备丢失诊断调用 [ID3D12Device5::RemoveDevice](https://learn.microsoft.com/en-us/windows/win32/api/d3d12/nf-d3d12-id3d12device5-removedevice)，真实执行同一设备的 removal 路径。本机硬件和 WARP 均通过：93 个已记录提交、92 个已确认 GPU 完成、1 个丢弃帧、恢复 1 次，重建后实际窗口颜色正确。移除时 fence 的 UINT64_MAX 值只用于识别丢失，不当作完成证据。每次 Run 最多恢复三次，超限或新设备创建失败返回错误。

两种 Mac CI 的 Metal 变化文本场景均通过 92 帧、一次 draw call、一页 R8 图集；CPU 编码 P95 在该轮 ARM64 / Intel runner 上分别为 4.196 / 6.325 ms。实际 drawable 的 F/组合重音/emoji mask 与 CoreText 对照 IoU=1，中文约 0.947、阿拉伯文约 0.859、office 约 0.926，边界最多相差一个物理像素。数据与 GPU/参考 PNG 保存为 CI artifacts；这些 runner 数值不代表其他设备，也不构成与 GPUI 的同机对比。

变化文本场景包含 2048 个矩形和 32 个每帧更新的标签，本机硬件/WARP 均通过 92 帧：三个槽全使用、submitted = completed、一次 draw call、空闲期间提交和时钟计数不变、Dispatch 重新唤醒。14 个字形共享一页 1 MiB R8 图集，而不是为每种字符串创建纹理。图集更新的累计上传量另外报告，不混入实例数 × 80 的实例上传断言。

大字号场景改变字体大小，迫使活动缓存达到 16 页后淘汰；本机硬件验证产生 9 次淘汰、1339 次光栅化和 41509 次命中，页面峰值 16 MiB。旧页由在途帧保留，暂存与资源屏障保证 GPU 读取生命周期。验收上限为当前缓存 16 MiB、包含在途代数的峰值 64 MiB；同尺寸 CPU 镜像和每帧上传暂存另占内存，页面字节指标不是进程总内存。压力使用调试层/GPU 校验，不将其时间当作无插桩性能。

原生窗口像素测试仅在 `GODESKTOP_READBACK=1` 时启用实际 swapchain 的 GPU copy，fence 完成后通过 PID/HWND 标识与序列号映射读回。测试核对 D3D12 后端属性、颜色/透明/裁剪、中文、分解/预组合音标、单色 emoji、阿拉伯文可见字形和文字裁剪；不使用 GDI 或桌面截图替代输出。可见字形检查不等于完整双向文字或彩色字体正确性，相关回归仍需补充。

该报告的 CPU 时间为构建至 commit/present 的单调时钟耗时，并分别报告视图/布局、drawable 获取及 GPU 编码的 P95，以区分计算和呈现资源等待。CAMetalDisplayLink 在回调前提供 drawable，所以该路径的 acquire 时间只包含渲染附件配置，不包含系统在回调前的调度耗时。报告同时记录 frame clock、请求/回调/合并/暂停计数和空闲前后快照。它不是线程 CPU 占用或完整输入到显示延迟；CI 数据不代表全部 Mac、Windows 设备或与 GPUI 的同机性能对比。上述剩余项目完成并分别验收后，才能认为现代绘制目标完成。
