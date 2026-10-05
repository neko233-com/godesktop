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

原生像素测试需要可用桌面会话。自动化通过 Win32 消息进入真实 WindowProc，不覆盖物理键鼠驱动、IME、所有字体、所有缩放比例、所有显卡或所有 Windows 客户端版本。Windows 10/11 的真实设备和多显示器测试仍应按发行版本补充；CI 的 Server 内核验证不等同于每台客户端设备验证。
