# GPU 绘制目标与验收

用户目标是使用现代原生绘制技术，并参考 Zed/GPUI 的架构。当前仍需把 Windows 默认 Direct2D 路径替换为 Direct3D 12；不能仅依据 Metal 改造、DXIL 编译或旧路径的绿灯判定整个目标完成。

## 参考与技术选择

Zed 的 [Windows 报告](https://zed.dev/blog/windows-progress-report) 说明了 DirectX 11 自定义 HLSL、DirectWrite 字形光栅化、字形图集和显存问题。[Metal 优化报告](https://zed.dev/blog/120fps) 解释了多帧资源池和异步 CPU/GPU 工作。本项目参考这些设计，独立实现 Go 布局/C ABI 与原生 GPU 后端。

- Windows：Direct3D 12 显式队列、资源状态、fence、flip model swapchain，自定义 DXIL 着色器；DirectWrite 负责字体与字形。
- macOS：Metal 实例化绘制、相邻批次、三个独立上传缓冲、完成回调；CoreText 负责字体与字形。
- 共享：80 字节实例 ABI、GPU 顶点生成、片元裁剪、圆角/线段距离函数、预乘 alpha 与画家顺序。

着色器 API 的版本号不会单独证明性能。运行路径、异步资源所有权、字形复用、内存上限和真实 GPU 帧验证都必须有证据。

## 当前证据和剩余工作

| 项目 | 当前状态 | 必需证据 |
| --- | --- | --- |
| Metal 实例化/合批 | 已接入默认路径 | 两种 Mac CI 的 2048 矩形 + 32 文本压力场景，draw calls <= 2，上传量 = 实例数 × 80 |
| Metal 异步三帧资源环 | 已接入默认路径 | 三槽都被使用，最多三个在途帧，正常绘制显式 buffer waits = 0，退出时 submitted = completed |
| GPU 诊断 | 已实现原生计数与 CPU/GPU 时间 | 原生压力报告；不能以 Go view 次数代替 GPU 完成数 |
| Direct3D 12 着色器 | DXC 编译/校验基础已加入 | 固定编译器、源文件和实例 ABI 的可复现 DXIL；仍需实际 GPU 执行 |
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
CGO_ENABLED=1 go run ./internal/renderstress -require-backend metal -output metal-render-stress.json
```

该报告的 CPU 时间为构建至 commit 的单调时钟耗时，并分别报告视图/布局、drawable 获取及 GPU 编码的 P95，以区分计算和呈现资源等待。它不是线程 CPU 占用或完整输入到显示延迟；CI 数据不代表全部 Mac、Windows 设备或与 GPUI 的同机性能对比。上述剩余项目完成并分别验收后，才能认为现代绘制目标完成。
