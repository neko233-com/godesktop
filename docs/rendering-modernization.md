# GPU 绘制目标与验收

用户目标是使用现代原生绘制技术，并参考 Zed/GPUI 的架构。当前仍需把 Windows 默认 Direct2D 路径替换为 Direct3D 12；不能仅依据 Metal 改造、DXIL 编译或旧路径的绿灯判定整个目标完成。

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
| Direct3D 12 着色器 | 可复现编译；离屏 D3D12 设备管线已加入 | RTX 5070 Ti 与 WARP 已通过 90 帧 × 16 个 GPU 像素检查；[Windows 2022/2025 CI 均通过](https://github.com/neko233-com/godesktop/actions/runs/37362077757)；仍需接入默认窗口后端 |
| Windows 默认 Direct3D 12 | 尚未实现 | HWND 与 swapchain 的真实绘制、fence 生命周期、缩放/设备重建、GPU 像素读回 |
| 两平台共享字形图集 | 尚未实现 | glyph/font/size/scale 缓存、复用计数、字节上限、跨帧生命周期、Unicode 像素验证 |
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
CGO_ENABLED=1 go run ./internal/renderstress -require-backend metal -require-frame-clock cametaldisplaylink -output metal-render-stress.json
```

Direct3D 12 设备和像素验收：

```powershell
$env:CGO_ENABLED = '1'
go run ./internal/dx12test -require-hardware -output .cache/dx12-hardware
go run ./internal/dx12test -warp -output .cache/dx12-warp
go run ./internal/dx12test -frames 7 -debug=false -output .cache/dx12-uninstrumented
```

该验证实际运行仓库中的预编译 DXIL，并在 fence 完成后从 GPU render target 复制 BGRA 像素。每帧检查实例步长、变化的颜色、画家顺序、透明混合、裁剪、圆角及抗锯齿、水平/对角线、R8 覆盖率纹理。第一个三帧批次通过独立 queue gate 延迟 GPU 执行，断言第四次尝试不能覆盖任何在途 allocator/上传缓冲，再释放 gate 并验证三帧各自的颜色。readback 的等待属于诊断过程；正常 submit 遇到未完成 fence 时只返回 defer。

默认设备优先选择支持 SM6 的硬件，缺少硬件时可使用 WARP；报告明确记录 software、adapter 和 debug/GPU validation 是否可用。`-require-hardware` 拒绝 WARP，`-warp` 显式验证软件设备，`-require-debug-layer` 强制要求调试层。PNG 只包含离屏 GPU 验证场景。R8 纹理目前使用合成覆盖率数据，不能据此宣称已完成字体图集、窗口 swapchain、输入到显示延迟或 gocode 的默认 D3D12 迁移。

设备报告同时保存 vendor/device ID 和原始 DXGI flags。[微软 DXGI 文档](https://learn.microsoft.com/en-us/windows/win32/direct3ddxgi/d3d10-graphics-programming-guide-dxgi) 指出，主 Basic Render 适配器可能不设置 SOFTWARE 标志；`0x1414:0x008c` 仍按软件适配器识别并从硬件候选中排除。GitHub Windows runner 的 Basic Render/WARP 结果不能当作物理 GPU 性能数据；本机硬件证据来自明确要求硬件的 RTX 5070 Ti 验收。

7 帧验收关闭调试层，另外覆盖不启用 GPU 校验插桩时的同一着色器管线，以及三帧批次后剩余一帧的读回路径。默认和 WARP 的 90 帧验收继续覆盖帧资源的多轮复用。

该报告的 CPU 时间为构建至 commit/present 的单调时钟耗时，并分别报告视图/布局、drawable 获取及 GPU 编码的 P95，以区分计算和呈现资源等待。CAMetalDisplayLink 在回调前提供 drawable，所以该路径的 acquire 时间只包含渲染附件配置，不包含系统在回调前的调度耗时。报告同时记录 frame clock、请求/回调/合并/暂停计数和空闲前后快照。它不是线程 CPU 占用或完整输入到显示延迟；CI 数据不代表全部 Mac、Windows 设备或与 GPUI 的同机性能对比。上述剩余项目完成并分别验收后，才能认为现代绘制目标完成。
