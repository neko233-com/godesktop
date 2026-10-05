//go:build windows && cgo

package platform

/*
#cgo CXXFLAGS: -std=c++17 -O2
#cgo LDFLAGS: -static -ldwrite -ld3d12 -ldxgi -lole32 -luuid -lgdi32 -luser32 -lstdc++
*/
import "C"
