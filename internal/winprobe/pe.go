package winprobe

import (
	"debug/pe"
	"fmt"
	"strings"
)

// ValidateAMD64PE rejects the wrong architecture and non-system runtime DLLs.
func ValidateAMD64PE(path string) error {
	file, err := pe.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if file.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return fmt.Errorf("PE machine is 0x%x; expected AMD64 (0x8664)", file.Machine)
	}
	if _, ok := file.OptionalHeader.(*pe.OptionalHeader64); !ok {
		return fmt.Errorf("expected PE32+ 64-bit optional header")
	}
	symbols, err := file.ImportedSymbols()
	if err != nil {
		return err
	}
	allowed := map[string]bool{"d3d12.dll": true, "dxgi.dll": true, "dwrite.dll": true, "kernel32.dll": true, "user32.dll": true, "gdi32.dll": true, "ole32.dll": true, "advapi32.dll": true, "shell32.dll": true, "ntdll.dll": true, "msvcrt.dll": true, "ucrtbase.dll": true}
	seen := make(map[string]bool)
	for _, symbol := range symbols {
		_, library, found := strings.Cut(symbol, ":")
		if !found {
			return fmt.Errorf("invalid imported symbol %q", symbol)
		}
		library = strings.ToLower(library)
		if !allowed[library] && !strings.HasPrefix(library, "api-ms-win-") && !strings.HasPrefix(library, "ext-ms-win-") {
			return fmt.Errorf("non-system DLL dependency: %s", library)
		}
		seen[library] = true
	}
	if !seen["d3d12.dll"] || !seen["dxgi.dll"] || !seen["dwrite.dll"] {
		return fmt.Errorf("native Direct3D 12/DXGI/DirectWrite imports are missing")
	}
	return nil
}
