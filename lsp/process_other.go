//go:build !windows

package lsp

import "os/exec"

func hideServerWindow(*exec.Cmd) {}
