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
| Direct2D | 实际客户端像素检查不透明颜色、半透明混合和父级裁剪；Unicode/组合字符/emoji 的平台文字测量 |
| 生命周期 | 同一进程三次启动和关闭窗口；关闭后 Context 拒绝调度；窗口缩放、最小化/恢复、空视图、重复键、嵌套 Run 拒绝、视图/点击/调度 panic 的错误返回 |
| 分发 | PE Machine=AMD64、PE32+；DLL 导入仅来自系统；普通 EXE 和 `-H=windowsgui` EXE 均完成两次原生绘制与状态调度 |

`native_windows_test.go` 的测试驱动只操作由测试进程 PID 和唯一标题确认的窗口，不发送全局键鼠输入。集成测试通过 stdin probe 在 UI 线程取得状态快照，避免用固定睡眠推断点击是否生效。消息调用、进程退出和 fixture 都有超时。

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

两种 Mac 架构还运行 `internal/renderstress`：持续绘制至少 90 个实际完成的 GPU 帧，检查三个上传缓冲都被使用、在途帧不超过三、正常提交没有显式缓冲等待、退出时所有提交完成。场景包含 2048 个圆角矩形和 32 个共享文本命令，要求最多两次 draw call、每实例 80 字节上传，并保存原生 CPU 编码 P50/P95。该验证证明执行了新的 Metal 实例化/异步路径，不证明全部像素正确或 GPUI 同机性能；后续 GPU 读回与共享字形图集仍需覆盖。

Windows CI 使用 `scripts/validate-shaders.ps1` 检查预编译 DXIL 与 HLSL/实例 ABI 一致。固定微软 DXC 版本和下载 SHA256，避免应用构建依赖网络或运行时编译器。该步骤只是 Direct3D 12 后端的编译基础；Windows 当前默认路径仍是 Direct2D，不将此编译成功当作现代后端执行成功。

工作区新增测试覆盖零基准 Flex、字体缓存隔离、矢量图标、标题拖动区域和窗口按钮生命周期。扩展测试通过真实 VSIX/Node 进程检查安装路径约束、大小限制、版本排序、并发命令、输出/文档事件、未知 API 错误和死循环超时。Node.js 是扩展测试的必需工具，CI 显式安装。

自定义标题栏窗口的初始尺寸限制到 Windows 显示器工作区。原生回归测试请求 10000×10000 DIP 窗口，检查实际窗口不越过工作区；最大化后检查客户区与工作区完全一致，并验证正常、最大化、还原三个状态下的底部状态栏像素和连续截图，避免小屏幕或不可见边框裁掉窗口内容。

`gocode` 子仓库有自己的五平台 CI 和 Windows 脚本：真实窗口测试文件选择、中文/emoji 输入、保存、扩展命令、标题栏命中、窗口缩放/关闭和 PE amd64；原生 smoke 同时要求真实绘制和已安装 VSIX 命令执行。截图和构建产物保存在其 Actions artifacts。父仓库 `go test ./...` 不会自动进入独立子模块，应分别验证两个仓库。

原生像素测试需要可用桌面会话。自动化通过 Win32 消息进入真实 WindowProc，不覆盖物理键鼠驱动、IME、所有字体、所有缩放比例、所有显卡或所有 Windows 客户端版本。Windows 10/11 的真实设备和多显示器测试仍应按发行版本补充；CI 的 Server 内核验证不等同于每台客户端设备验证。
