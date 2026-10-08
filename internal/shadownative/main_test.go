//go:build (windows || darwin) && cgo

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvidenceResetKeepsUnrelatedOutput(t *testing.T) {
	output := t.TempDir()
	for _, name := range []string{"current.json", "failed.json", "run-0.png", "run-1.png", "run-2.png", "shadow-native.exe", "unrelated.txt"} {
		if err := os.WriteFile(filepath.Join(output, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := resetEvidence(output); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 2 {
		t.Fatalf("fixed evidence reset removed unrelated files: %v, %v", entries, err)
	}
	for _, name := range []string{"shadow-native.exe", "unrelated.txt"} {
		data, err := os.ReadFile(filepath.Join(output, name))
		if err != nil || string(data) != name {
			t.Fatalf("unrelated %s changed: %q, %v", name, data, err)
		}
	}
}

func TestOwnedTempRejectsOutsideArtifactRoot(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "new-output")
	if _, err := prepareOwnedTemp(outside); err == nil {
		t.Fatal("external output was accepted")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("external output was created: %v", err)
	}
}

func TestTinyGeometryKeepsFractionalDeviceSamples(t *testing.T) {
	for _, density := range []float64{1, 1.5, 2} {
		for _, caster := range tinyGeometry(density) {
			value, err := tinyReference(caster.spec, caster.queryX, caster.queryY)
			if err != nil || value < .03 || value > .97 {
				t.Errorf("%s at%g fails fractional negative control: coverage%g error%v deviceRect%+v", caster.name, density, value, err, caster.spec.Rect)
			}
		}
	}
}
