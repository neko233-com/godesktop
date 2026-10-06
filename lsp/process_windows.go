//go:build windows

package lsp

import (
	"os/exec"
	"syscall"
)

func hideServerWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
