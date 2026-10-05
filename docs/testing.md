# 自动化验证

Windows 的支持目标是 **x86-64 / amd64，GOAMD64=v1**。x86 在本项目文档中不表示 32 位 `386`。Windows 原生构建需要 64 位 MinGW-w64、Go 1.27 和 cgo。

## Windows 一键验证

```powershell
powershell -ExecutionPolicy Bypass -File scripts/test-windows-amd64.ps1
```

脚本检查工具链架构，临时设置 `GOOS=windows`、`GOARCH=amd64`、`GOAMD64=v1` 和 `GOEXPERIMENT=cgocheck2`，结束后恢复调用进程的环境变量。默认执行三轮随机顺序的 race 测试和两个各五秒的 fuzz 任务，检查普通 EXE 与 GUI 子系统 EXE，并保存覆盖率到 `.cache/windows-validation/<run-id>/`。

| 层次 | 自动验证的行为 |
| --- | --- |
| Go 核心 | Row/Column 固有尺寸、Grow、间距、显式尺寸、空树、裁剪、文本溢出、缓存边界、禁用按钮、焦点环绕、指针取消、旧目标清理、FIFO 调度与关闭竞争 |
| 随机测试 | 有限矩形的交集性质；随机元素树的裁剪范围、可点击区域有效性 |
| C ABI | C/C++ 编译时验证 float/int、矩形、颜色、命令尺寸与文本偏移一致 |
| Windows 原生 | 独立进程创建真实 HWND；通过 Win32 消息检查点击、跨按钮/窗口外释放、失焦取消、Tab、Enter、Space、按键重复过滤、禁用按钮 |
| Direct3D 12 窗口 | 从 fence 完成的 swapchain GPU copy 检查不透明颜色、半透明混合和裁剪；核对 HWND 的 D3D12 后端；中文、音标、单色 emoji、阿拉伯文可见字形与文字裁剪；分解/预组合音标像素相同 |
| 生命周期 | 同一进程三次启动和关闭窗口；关闭后 Context 拒绝调度；窗口缩放、最小化/恢复、空视图、重复键、嵌套 Run 拒绝、视图/点击/调度 panic 的错误返回 |
| 分发 | PE Machine=AMD64、PE32+；DLL 导入仅来自系统；普通 EXE 和 `-H=windowsgui` EXE 均完成两次原生绘制与状态调度 |

`native_windows_test.go` 的测试驱动只操作由测试进程 PID 和唯一标题确认的窗口，不发送全局键鼠输入。集成测试通过 stdin probe 在 UI 线程取得状态快照，避免用固定睡眠推断点击是否生效。消息调用、进程退出和 fixture 都有超时。

尺寸变化后的立即点击回归在同一个 UI 输入回调内连续改变客户区宽度六次，每次立刻点击右侧锚定按钮，最后最小化/恢复并再次点击。回调期间不允许新 GPU 提交，七次点击都必须命中当前布局；未修复桥接的负向对照只能命中四次。生产桥接在尺寸或 DPI 变化后使布局失效，指针事件先同步更新 Go 命中区域，再由 DXGI 正常节奏绘制。

## 覆盖率

核心测试与原生 fixture 的 Go 覆盖率通过 `go tool covdata` 合并；根包低于 **90%** 会让 Windows 验证失败。fixture 也启用 race 与严格 cgo 检查。覆盖率指 Go 语句，不能代表 C++ 图形驱动覆盖率；C++ 桥接通过原生行为、像素和 ABI 检查验证。

本机首次完整数据：根包 **99.2%**，Go 原生适配层 **92.7%**。这些数值对应加入本测试套件时的代码，后续以 CI 保存的 `coverage.txt` 为准。

```powershell
# 较快地运行一轮完整检查
powershell -File scripts/test-windows-amd64.ps1 -Repeat 1 -FuzzSeconds 2

# 仅运行核心与原生集成测试
$env:CGO_ENABLED = '1'
$env:GOEXPERIMENT = 'cgocheck2'
go test -race -shuffle=on ./...

# 单独检查发布 EXE 架构和依赖
go run ./internal/pecheck ./bin/counter.exe
```

## CI 与验证边界

CI 使用 Windows Server 2022 和 2025 的 amd64 runner 执行上述完整脚本；macOS Intel / Apple Silicon 执行三轮 race、严格 cgo、编译和原生 smoke；Ubuntu 执行核心 race、fuzz 和无 cgo 回退检查。Windows 报告和两种 EXE 都上传为 artifacts，失败时也保留已生成的报告。

两种 Mac 架构还运行 `internal/renderstress`：持续绘制至少 90 个实际完成的 GPU 帧，检查三个上传缓冲都被使用、在途帧不超过三、正常提交没有显式缓冲等待、退出时所有提交完成。变化文本场景包含 2048 个圆角矩形和 32 个标签，要求最多两次 draw call、每实例 80 字节上传、最多 14 个光栅化字形和一页 1 MiB R8 图集，并保存原生 CPU 编码 P50/P95。另行运行大字号缓存淘汰，检查活动/在途页面上限和提交完成。

`internal/metaltest` 在同一进程启动两次真实 AppKit 窗口，从实际 drawable 在 GPU 完成后复制像素。几何样本检查透明混合、矩形裁剪、圆角和斜线；文字与独立 CoreText 整行绘制比较方向、位置和 ink mask，覆盖中文、单色 emoji、阿拉伯文、连字、组合/预组合重音和 shader 裁剪。单字形光栅化与整行亚像素相位可能改变边缘，验收要求 mask IoU >= 0.60、边界差 <= 2 DIP × scale 对应的物理像素；重音对照另要求近乎相同覆盖率。实际 GPU PNG、参考 PNG 和 JSON 随失败/成功 artifacts 保存。CI 显示器目前 scale=1；此验证不代表全部字体、缩放或完整文本编辑输入。

Windows CI 使用 `scripts/validate-shaders.ps1` 检查预编译 DXIL 与 HLSL/实例 ABI 一致。固定微软 DXC 版本和下载 SHA256，避免应用构建依赖网络或运行时编译器。默认 HWND 已使用 D3D12；CI 分别验证默认适配器和 WARP 的实际窗口压力、字形复用、DXGI 节奏、空闲唤醒以及 WARP 原生输入/像素。PE 检查要求 D3D12/DXGI/DirectWrite 系统导入并拒绝 Direct2D 依赖。

工作区新增测试覆盖零基准 Flex、字体缓存隔离、矢量图标、标题拖动区域和窗口按钮生命周期。扩展测试通过真实 VSIX/Node 进程检查安装路径约束、大小限制、版本排序、并发命令、输出/文档事件、未知 API 错误和死循环超时。Node.js 是扩展测试的必需工具，CI 显式安装。

Windows 的 `internal/renderstress -device-recovery` 在第九次提交后调用该 renderer 的 ID3D12Device5::RemoveDevice，实际使 fence 进入 removed 状态，然后继续同一 HWND/Go 状态绘制并读回正确像素。硬件与 WARP 分别验证恢复一次、丢弃 1–3 个未确认帧、后续至少 90 个真实 GPU 完成、submitted = completed + dropped、空闲暂停/唤醒以及重建后的 1 MiB 图集。普通场景仍要求 submitted = completed；诊断报告不会把丢弃帧算成完成帧。该调用不触发全系统 TDR。

`-completion-race` 使用同一真实提交路径，在 CPU 收集前主动等待 GPU fence 完成，然后调用生产 completion watcher，要求它仍返回已经完成但尚未收集的提交事件。这样可稳定覆盖 GPU 恰好在 poll 与监听注册之间完成的时序，避免消息循环空闲时漏收最后一帧；普通压力场景继续验证零显式缓冲等待。这个单独诊断场景的同步等待不属于正常绘制路径，报告明确标记 diagnostic_completion_race。

离屏 DXIL 验收还保留一次真实旧 fence 的异步通知，把后一提交阻塞在独立 queue gate 后，再执行生产同步等待。旧通知不能使关闭/缩放/读回等待提前成功或报错，目标 fence 必须真的完成。该诊断在计时帧前保持队列 100 ms，并在报告中明确标记；不计入逐帧 CPU/GPU 样本。同步等待与消息循环观察使用独立事件，并始终依据 fence 数值检查完成，在五秒总期限内处理通知。

Mac CI 的 `-metal-recovery` 在真实 GPU 完成后注入资源恢复请求，要求同一 NSWindow、Go 状态/Dispatch 保留、恢复后至少 90 帧、字形图集重新建立及实际 drawable 像素。`internal/metaltest` 还请求四次恢复，验证三次上限、错误返回、队列排空和失败后重新 Run 的计数/快照隔离。报告标记诊断注入；不伪造 command-buffer status、完成数或硬件断开。eGPU 移除、权限撤销与系统 GPU 故障需另行在具备这些条件的真实设备上验证。

两种 Mac 还用 `internal/metaltest -drawable-scale 1.5` / `2` 指定实际窗口 drawable 的诊断像素密度，保持正常视图坐标并核对 GPU 图像尺寸、字体与裁剪，再执行同一恢复上限和重新启动场景。这覆盖分数/整数 scale 的 GPU 坐标与字形缓存；默认场景继续使用系统自动密度。诊断不改变系统缩放或显示器硬件，真实 Retina/跨显示器迁移仍应补充。

Windows 的 `internal/inputlatency` 在当前进程的已核验 HWND 注入 40 次 WM_CHAR，Go 输入事件逐次更新唯一颜色，再读取实际已完成 GPU 像素。每个更新必须在一秒内被观察、输入状态准确递增、退出提交排空且没有丢弃/恢复。JSON 保存完整样本、P50/P95 和 1 ms 轮询间隔；它测量消息至完成像素的观察耗时，包含轮询，不覆盖物理键鼠或屏幕扫描。CI 关闭 GPU debug 插桩运行该场景，报告同时记录请求的适配器模式。

自定义标题栏窗口的初始尺寸限制到 Windows 显示器工作区。原生回归测试请求 10000×10000 DIP 窗口，检查实际窗口不越过工作区；最大化后检查客户区与工作区完全一致，并验证正常、最大化、还原三个状态下的底部状态栏像素和连续截图，避免小屏幕或不可见边框裁掉窗口内容。

`gocode` 子仓库有自己的五平台 CI 和 Windows 脚本：真实窗口测试文件选择、中文/emoji 输入、保存、扩展命令、标题栏命中、窗口缩放/关闭和 PE amd64；原生 smoke 同时要求真实绘制和已安装 VSIX 命令执行。截图和构建产物保存在其 Actions artifacts。父仓库 `go test ./...` 不会自动进入独立子模块，应分别验证两个仓库。

原生像素测试需要可用桌面会话。自动化通过 Win32 消息进入真实 WindowProc，不覆盖物理键鼠驱动、IME、所有字体、所有缩放比例、所有显卡或所有 Windows 客户端版本。Windows 10/11 的真实设备和多显示器测试仍应按发行版本补充；CI 的 Server 内核验证不等同于每台客户端设备验证。
