//go:build windows

// guismoke launches a GUI-subsystem EXE and reveals only its own test window.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/neko233-com/godesktop/internal/winprobe"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: guismoke counter-gui.exe")
	}
	command := exec.Command(os.Args[1], "-smoke")
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Start(); err != nil {
		log.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(25 * time.Second)
	defer deadline.Stop()
	revealed := false
	for {
		select {
		case err := <-done:
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("GUI subsystem smoke passed")
			return
		case <-ticker.C:
			if !revealed {
				if window, err := winprobe.Find("godesktop · Go 1.27", uint32(command.Process.Pid)); err == nil {
					window.Show(9)
					revealed = true
				}
			}
		case <-deadline.C:
			_ = command.Process.Kill()
			<-done
			log.Fatal("GUI subsystem smoke timed out")
		}
	}
}
