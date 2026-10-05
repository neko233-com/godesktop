//go:build windows && cgo

package platform

/*
#cgo CXXFLAGS: -std=c++17 -O2
#cgo LDFLAGS: -static -ld2d1 -ldwrite -lole32 -luuid -lgdi32 -luser32 -lstdc++
*/
import "C"
