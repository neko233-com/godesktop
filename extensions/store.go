// Package extensions installs local VSIX archives and runs a documented subset
// of the VS Code extension API in a separate Node.js process.
package extensions

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Command struct {
	Command string `json:"command"`
	Title   string `json:"title"`
}
type Manifest struct {
	Name             string   `json:"name"`
	Publisher        string   `json:"publisher"`
	Version          string   `json:"version"`
	DisplayName      string   `json:"displayName"`
	Description      string   `json:"description"`
	Main             string   `json:"main"`
	ActivationEvents []string `json:"activationEvents"`
	Contributes      struct {
		Commands []Command `json:"commands"`
	} `json:"contributes"`
}
type Extension struct {
	Manifest Manifest `json:"manifest"`
	Path     string   `json:"path"`
}

func (e Extension) ID() string { return e.Manifest.Publisher + "." + e.Manifest.Name }

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)
var version = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func validate(m Manifest) error {
	if !identifier.MatchString(m.Name) || !identifier.MatchString(m.Publisher) || !version.MatchString(m.Version) {
		return errors.New("extension requires a valid publisher, name and numeric major.minor.patch version")
	}
	for _, part := range strings.Split(m.Version, ".") {
		if _, err := strconv.ParseUint(part, 10, 64); err != nil {
			return errors.New("extension version component is too large")
		}
	}
	if m.Main != "" {
		if _, err := localPath(m.Main); err != nil {
			return fmt.Errorf("main: %w", err)
		}
	}
	return nil
}

func localPath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(name, "./") {
		name = name[2:]
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || strings.ContainsAny(part, ":\x00") || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return "", fmt.Errorf("unsafe archive path %q", name)
		}
		base := strings.ToUpper(strings.Split(part, ".")[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return "", fmt.Errorf("reserved archive path %q", name)
		}
	}
	path := filepath.FromSlash(name)
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return path, nil
}

// Install extracts a VSIX into an immutable version directory. Archives are
// limited to 4096 entries, 16 MiB per file and 64 MiB of uncompressed data.
func Install(root, archive string) (Extension, error) {
	var result Extension
	z, err := zip.OpenReader(archive)
	if err != nil {
		return result, err
	}
	defer z.Close()
	if len(z.File) > 4096 {
		return result, errors.New("too many archive entries")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return result, err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return result, err
	}
	stage, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		return result, err
	}
	defer func() {
		// Stage is a direct child of the explicitly selected installation root.
		if filepath.Dir(stage) == root && strings.HasPrefix(filepath.Base(stage), ".install-") {
			_ = os.RemoveAll(stage)
		}
	}()
	seen := make(map[string]bool)
	var total uint64
	for _, file := range z.File {
		if !strings.HasPrefix(file.Name, "extension/") {
			continue
		}
		name := strings.TrimPrefix(file.Name, "extension/")
		if name == "" {
			continue
		}
		path, err := localPath(strings.TrimSuffix(name, "/"))
		if err != nil {
			return result, err
		}
		key := strings.ToLower(path)
		if seen[key] {
			return result, fmt.Errorf("duplicate archive path %q", name)
		}
		seen[key] = true
		if file.Mode()&os.ModeSymlink != 0 || (!file.FileInfo().IsDir() && !file.Mode().IsRegular()) {
			return result, errors.New("special archive entries are unsupported")
		}
		total += file.UncompressedSize64
		if file.UncompressedSize64 > 16<<20 || total > 64<<20 {
			return result, errors.New("archive exceeds extraction limits")
		}
		target := filepath.Join(stage, path)
		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(target, 0755); err != nil {
				return result, err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return result, err
		}
		r, err := file.Open()
		if err != nil {
			return result, err
		}
		w, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			r.Close()
			return result, err
		}
		n, copyErr := io.Copy(w, io.LimitReader(r, (16<<20)+1))
		closeErr := w.Close()
		readErr := r.Close()
		if err = errors.Join(copyErr, closeErr, readErr); err != nil {
			return result, err
		}
		if n > 16<<20 {
			return result, errors.New("archive file exceeds limit")
		}
	}
	data, err := os.ReadFile(filepath.Join(stage, "package.json"))
	if err != nil {
		return result, err
	}
	if len(data) > 256<<10 {
		return result, errors.New("manifest exceeds limit")
	}
	if err = json.Unmarshal(data, &result.Manifest); err != nil {
		return result, err
	}
	if err = validate(result.Manifest); err != nil {
		return result, err
	}
	result.Path = filepath.Join(root, strings.ToLower(result.ID())+"-"+result.Manifest.Version)
	if _, err = os.Stat(result.Path); err == nil {
		return result, errors.New("extension version already installed")
	} else if !os.IsNotExist(err) {
		return result, err
	}
	if err = os.Rename(stage, result.Path); err != nil {
		return result, err
	}
	return result, nil
}

// List returns the latest numeric version of each installed extension.
func List(root string) ([]Extension, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	latest := map[string]Extension{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path, err := filepath.Abs(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(filepath.Join(path, "package.json"))
		if err != nil {
			continue
		}
		var e Extension
		if json.Unmarshal(data, &e.Manifest) != nil || validate(e.Manifest) != nil {
			continue
		}
		e.Path = path
		id := strings.ToLower(e.ID())
		old, ok := latest[id]
		if !ok || newer(e.Manifest.Version, old.Manifest.Version) {
			latest[id] = e
		}
	}
	out := make([]Extension, 0, len(latest))
	for _, e := range latest {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out, nil
}
func newer(a, b string) bool {
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	for i := range 3 {
		p, _ := strconv.ParseUint(x[i], 10, 64)
		q, _ := strconv.ParseUint(y[i], 10, 64)
		if p != q {
			return p > q
		}
	}
	return false
}
