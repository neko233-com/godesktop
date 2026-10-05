// pecheck verifies that a distributable EXE uses the supported Windows ABI.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/neko233-com/godesktop/internal/winprobe"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: pecheck path/to/application.exe")
	}
	if err := winprobe.ValidateAMD64PE(os.Args[1]); err != nil {
		log.Fatal(err)
	}
	fmt.Println("PE check passed: AMD64 / PE32+ / system DLLs only")
}
