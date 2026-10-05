# 贡献

Go 1.27 是最低版本。原生构建需要 cgo 和目标系统工具链；Windows 使用 MinGW-w64，macOS 使用 Xcode Command Line Tools。Go API 与业务逻辑放在根包，操作系统细节留在 `internal/platform`。

更改布局、调度、指针捕获或焦点行为时，为具体行为添加核心测试。更改原生代码时运行 `examples/counter -smoke`，并在目标平台手动检查点击、Tab/Shift+Tab、Enter/Space、缩放和关闭窗口。不要从未运行的平台推断已经验证通过。

```sh
gofmt -w .
CGO_ENABLED=1 go test -race ./...
CGO_ENABLED=1 go vet ./...
CGO_ENABLED=1 go run ./examples/counter -smoke
actionlint .github/workflows/ci.yml
git diff --check
```

PowerShell 用户先设置 `$env:CGO_ENABLED = '1'`。工作流 lint 也可运行 `powershell -File scripts/validate-github-actions.ps1`；需要在 PATH 中安装 actionlint 和 ShellCheck。该脚本不自动下载工具。

性能变更应说明具体场景和限制。`BenchmarkLayout1000Elements` 只测已建立树的布局和命令生成，不包括视图构造、系统文字测量、cgo 传输或 GPU。不要把它当作完整 UI 的帧率。

提交前确认 `git status --short` 中只有预期源文件；二进制、profile 和本地缓存不进入版本库。CI 覆盖 Windows、macOS Intel/Apple Silicon 的原生构建及 smoke，Linux 运行可移植核心验证。
