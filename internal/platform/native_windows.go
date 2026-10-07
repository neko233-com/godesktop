//go:build windows && cgo

package platform

/*
#cgo CXXFLAGS: -std=c++17 -O2 -D_WIN32_WINNT=0x0A00 -DNTDDI_VERSION=0x0A000003
#cgo LDFLAGS: -static -ldwrite -ld3d12 -ld3d11 -ld2d1 -ldxgi -lole32 -luuid -lgdi32 -luser32 -lstdc++
*/
import "C"
