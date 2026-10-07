# GPU 绘制目标与验收

现代原生 GPU 管线已在 [v0.3.1](https://github.com/neko233-com/godesktop/releases/tag/v0.3.1) 发布，参考 Zed/GPUI 的架构。Windows 默认窗口采用 Direct3D 12/DirectWrite，macOS 采用 Metal/CoreText；两者均已接入 R8 字形图集、GPU 完成后复用的三帧资源环、显示同步调度及有界资源恢复。[发布提交 7322147 的五平台 CI](https://github.com/neko233-com/godesktop/actions/runs/37385691779) 全部通过。gocode 固定依赖这一公开模块版本，通过 GOWORK=off 独立构建和验证。

## 参考与技术选择

Zed 的 [Windows 报告](https://zed.dev/blog/windows-progress-report) 说明了 DirectX 11 自定义 HLSL、DirectWrite 字形光栅化、字形图集和显存问题。[Metal 优化报告](https://zed.dev/blog/120fps) 解释了多帧资源池和异步 CPU/GPU 工作。本项目参考这些设计，独立实现 Go 布局/C ABI 与原生 GPU 后端。

- Windows：Direct3D 12 显式队列、资源状态、fence、flip model swapchain，自定义 DXIL 着色器；DirectWrite 负责字体与字形。
- macOS：Metal 实例化绘制、相邻批次、三个独立上传缓冲、完成回调；CoreText 负责字体与字形。
- macOS 14+ 使用 [CAMetalDisplayLink](https://developer.apple.com/documentation/quartzcore/cametaldisplaylink) 提供的 drawable 和显示回调；合并重绘请求，空闲后暂停，Dispatch/输入重新唤醒。macOS 13 使用 MTKView 的显示同步循环，诊断明确报告该兼容路径。
- 共享：80 字节实例 ABI、GPU 顶点生成、片元裁剪、圆角/线段距离函数、预乘 alpha 与画家顺序。

着色器 API 的版本号不会单独证明性能。运行路径、异步资源所有权、字形复用、内存上限和真实 GPU 帧验证都必须有证据。

## 当前证据

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
| Windows 设备丢失恢复 | 本机硬件/WARP 及 [eb6eff0 两种 Windows CI 已通过](https://github.com/neko233-com/godesktop/actions/runs/37376434509) | 同一窗口自动重建、恢复后 92 个完成帧、1 个丢弃帧、正确 GPU 像素；另行强制完成通知竞态时序 |
| Metal 设备移除/提交恢复 | 已实现；[e8a262d 两种 Mac CI 已通过](https://github.com/neko233-com/godesktop/actions/runs/37378878913) | 同一窗口重建、Go 状态/Dispatch 保留、恢复后 92 帧及实际 drawable；三次恢复上限与失败后重新 Run；CI 注入不声称硬件拔除 |
| 绘制正确性与性能 | GPU 像素、字形/资源和原生 CPU 时间已验证；新增输入到完成像素测量 | 两 Mac 默认/1.5×/2× 实际 drawable；Windows 实际 swapchain/离屏 DXIL；本机 602 帧无插桩 CPU P50/P95 和 40 次 native 输入观察；不声称物理显示延迟或 GPUI 同机性能 |
| gocode 消费新后端 | 独立模块固定依赖 v0.3.1，无本地 replace | GOWORK=off 的三轮 Windows race/原生输入与 EXE smoke；[9bbd371 的五平台 CI](https://github.com/neko233-com/gocode/actions/runs/37386679226) |

## 预编译 Windows 着色器

后续彩色字体实现让普通 R8 和原生预乘 RGBA 字形共用固定 16 槽纹理表，混合文字与几何保持一次绘制，用户 bitmap 使用独立绑定。Windows 彩色 cache miss 使用 DirectWrite 枚举和原生 Direct2D 光栅化，D3D12 仍负责窗口绘制；较新系统运行时查询 COLRv1 paint API。缓存以实际 R8/RGBA 页字节计费。Mac 的公开 v0.11.0 已通过两种架构三种密度，Windows 候选的实际颜色／参考图／缩放／淘汰／恢复范围与发布状态见 [工程记录](../agent%20docs/color-glyphs.md)。

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
CGO_ENABLED=1 go run ./internal/renderstress -require-backend metal -require-frame-clock cametaldisplaylink -metal-recovery -output metal-recovery.json
CGO_ENABLED=1 go run ./internal/metaltest -drawable-scale 1.5 -output metal-pixels-150
CGO_ENABLED=1 go run ./internal/metaltest -drawable-scale 2 -output metal-pixels-200
```

Direct3D 12 设备和像素验收：

```powershell
$env:CGO_ENABLED = '1'
go run ./internal/dx12test -require-hardware -output .cache/dx12-hardware
go run ./internal/dx12test -warp -output .cache/dx12-warp
go run ./internal/dx12test -frames 7 -debug=false -output .cache/dx12-uninstrumented
```

该验证实际运行仓库中的预编译 DXIL，并在 fence 完成后从 GPU render target 复制 BGRA 像素。每帧检查实例步长、变化的颜色、画家顺序、透明混合、裁剪、圆角及抗锯齿、水平/对角线、R8 覆盖率纹理。第一个三帧批次通过独立 queue gate 延迟 GPU 执行，断言第四次尝试不能覆盖任何在途 allocator/上传缓冲，再释放 gate 并验证三帧各自的颜色。readback 的等待属于诊断过程；正常 submit 遇到未完成 fence 时只返回 defer。

默认设备优先选择支持 SM6 的硬件，缺少硬件时可使用 WARP；离屏报告明确记录 software、adapter 和 debug/GPU validation 是否可用。`-require-hardware` 拒绝 WARP，`-warp` 显式验证软件设备，`-require-debug-layer` 强制要求调试层。PNG 只包含离屏 GPU 验证场景；该夹具使用合成 R8 覆盖率，窗口字形图集由下述独立场景验证。

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

设备丢失诊断调用 [ID3D12Device5::RemoveDevice](https://learn.microsoft.com/en-us/windows/win32/api/d3d12/nf-d3d12-id3d12device5-removedevice)，真实执行同一设备的 removal 路径。本机硬件和 WARP 均通过：恢复前完成 8 帧、恢复后完成 92 帧，101 个已记录提交中 100 个已确认 GPU 完成、1 个丢弃帧，恢复 1 次，重建后实际窗口颜色正确。报告单独记录恢复前/后的完成数，验收至少 90 个恢复后完成帧。移除时 fence 的 UINT64_MAX 值只用于识别丢失，不当作完成证据。每次 Run 最多恢复三次，超限或新设备创建失败返回错误。

两种 Mac CI 的 Metal 变化文本场景均通过 92 帧、一次 draw call、一页 R8 图集；CPU 编码 P95 在该轮 ARM64 / Intel runner 上分别为 4.196 / 6.325 ms。实际 drawable 的 F/组合重音/emoji mask 与 CoreText 对照 IoU=1，中文约 0.947、阿拉伯文约 0.859、office 约 0.926，边界最多相差一个物理像素。数据与 GPU/参考 PNG 保存为 CI artifacts；这些 runner 数值不代表其他设备，也不构成与 GPUI 的同机对比。

Metal 恢复验证在真实 GPU 第九次成功完成后向生产恢复处理器发出诊断请求，验证队列/管线/帧槽/图集重新分配、NSWindow identity 保持、Go 状态及 Dispatch 继续生效、恢复后至少 90 个真实完成帧及 drawable 颜色。另行请求四次恢复，要求第三次之后返回上限错误，排空提交并能再次 Run；下一次的计数和快照独立。此注入不会物理断开 GPU，也不会将成功提交伪造为错误或丢弃帧。实际 eGPU 热拔插和系统 GPU 故障仍需真实硬件验收。

e8a262d 两种 Mac 的实际结果为恢复前 9 帧、恢复后 92 帧，共 101 次成功完成、1 次资源恢复、0 次丢弃，Go 模型更新至 11，图集恢复为一页 1 MiB。四次请求的上限测试在 36 次完成后返回预期错误，下一次 Run 的快照为第 8 帧、恢复/丢弃计数归零。

新增 1.5×/2× 密度诊断设置实际窗口 drawable 的像素尺寸，同时保持同一视图坐标和默认 GPU 绘制路径；GPU 读回必须反映请求的密度，并运行相同几何/Unicode 对照及恢复生命周期验收。默认测试仍使用系统自动选择的密度。该诊断验证 GPU 坐标和字形 scale，不声称改变了显示器硬件或系统缩放配置。

两种 Mac 在 d51d6ec 上的 1.5×/2× 验收均通过，实际 drawable 的 scale 分别为 1.5 / 2，Unicode mask 最低 IoU 分别约 0.753 / 0.901、边界最多相差一个物理像素；同样完成恢复上限与重新 Run。本机 RTX 硬件、关闭调试插桩的 602 帧变化文本场景，CPU P50/P95 为 0.891 / 1.450 ms，光栅化仍为 14 个字形、192626 次命中、图集保持 1 MiB。

`internal/inputlatency` 从当前进程拥有的 HWND 注入 40 次 WM_CHAR，经 Go 输入/状态更新和默认 D3D12 绘制，再观察 fence 完成后像素。每次颜色唯一，旧帧无法满足下一次观察；报告保存所有样本、P50/P95 和 1 ms 消费端轮询间隔。本机硬件关闭调试层的此次 P50/P95 为 16.675 / 17.552 ms。测量包含 SendMessage 和 CPU 轮询，不能当作物理键盘或显示扫描延迟；CI 默认/WARP 结果仅证明路径和正确性，不能充当物理 GPU 性能。

```powershell
$env:CGO_ENABLED = '1'
$env:GODESKTOP_GPU_ADAPTER = 'hardware'
$env:GODESKTOP_GPU_DEBUG = '0'
go run -race ./internal/inputlatency -output .cache/dx12-input-latency-hardware.json
```

变化文本场景包含 2048 个矩形和 32 个每帧更新的标签，本机硬件/WARP 均通过 92 帧：三个槽全使用、submitted = completed、一次 draw call、空闲期间提交和时钟计数不变、Dispatch 重新唤醒。14 个字形共享一页 1 MiB R8 图集，而不是为每种字符串创建纹理。图集更新的累计上传量另外报告，不混入实例数 × 80 的实例上传断言。

发布提交 3c7cd22 上重新执行 RTX 5070 Ti 无调试插桩验收：602 个提交全部完成，CPU P50/P95 为 0.881/1.390 ms，14 次光栅化、192626 次命中和 1 MiB 活动图集；40 次输入观察 P50/P95 为 16.625/17.278 ms。发布附件 `godesktop-v0.3.0-validation.zip` 保存该提交、复现命令、适配器报告、完整输入样本、GPU PNG 和 JSON；这些数值是该次本机结果。

独立 gocode 的 Windows 2025 CI 后续发现 v0.3.0 关闭时可能消费旧异步 fence 通知并错误返回。v0.3.1 将同步等待与消息循环通知隔离，使用五秒总期限和真实 fence 数值判断完成；保持旧通知并阻塞后一提交的负向对照可复现原错误，硬件/WARP 修复回归与新五平台 CI 均通过。`godesktop-v0.3.1-validation.zip` 保存发布提交、复现命令及两种设备的报告和 GPU PNG。旧标签保留，推荐使用 v0.3.1。

大字号场景改变字体大小，迫使活动缓存达到 16 页后淘汰；本机硬件验证产生 9 次淘汰、1339 次光栅化和 41509 次命中，页面峰值 16 MiB。旧页由在途帧保留，暂存与资源屏障保证 GPU 读取生命周期。验收上限为当前缓存 16 MiB、包含在途代数的峰值 64 MiB；同尺寸 CPU 镜像和每帧上传暂存另占内存，页面字节指标不是进程总内存。压力使用调试层/GPU 校验，不将其时间当作无插桩性能。

原生窗口像素测试仅在 `GODESKTOP_READBACK=1` 时启用实际 swapchain 的 GPU copy，fence 完成后通过 PID/HWND 标识与序列号映射读回。测试核对 D3D12 后端属性、颜色/透明/裁剪、中文、分解/预组合音标、单色 emoji、阿拉伯文可见字形和文字裁剪；不使用 GDI 或桌面截图替代输出。可见字形检查不等于完整双向文字或彩色字体正确性，相关回归仍需补充。

该报告的 CPU 时间为构建至 commit/present 的单调时钟耗时，并分别报告视图/布局、drawable 获取及 GPU 编码的 P95，以区分计算和呈现资源等待。CAMetalDisplayLink 在回调前提供 drawable，所以该路径的 acquire 时间只包含渲染附件配置，不包含系统在回调前的调度耗时。报告同时记录 frame clock、请求/回调/合并/暂停计数和空闲前后快照。它不是线程 CPU 占用或完整输入到显示延迟；CI 数据不代表全部 Mac、Windows 设备或与 GPUI 的同机性能对比。IME、无障碍、编辑器能力和更广的真实设备测试仍按路线图继续推进。
