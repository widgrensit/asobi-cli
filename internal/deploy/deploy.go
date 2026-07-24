package deploy

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widgrensit/asobi-cli/internal/client"
)

// zipEpoch is a fixed, valid DOS-representable timestamp. Stamping every
// entry with it keeps bundles byte-reproducible and avoids the invalid
// 1980-00-00 date that a zero mtime encodes to (asobi-cli#34).
var zipEpoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

func CollectScripts(dir string) ([]client.Script, error) {
	var scripts []client.Script

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".lua") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("rel path %s: %w", path, err)
		}

		scripts = append(scripts, client.Script{
			// Zip entry names must use forward slashes (APPNOTE 4.4.17). On
			// Windows filepath.Rel yields backslashes, which the engine then
			// writes as literal backslash-named files (config.lua/match.lua
			// never found -> guest_auth off, modes don't load). Normalise.
			Path:    filepath.ToSlash(rel),
			Content: string(content),
		})
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}

	return scripts, nil
}

// ZipScripts creates a zip archive from collected scripts.
func ZipScripts(scripts []client.Script) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, s := range scripts {
		f, err := w.CreateHeader(&zip.FileHeader{
			// Enforce forward-slash entry names (APPNOTE 4.4.17) regardless of
			// how Path was built, so a Windows-authored path never ships a
			// backslash entry the engine can't resolve. Cross-platform, unlike
			// filepath.ToSlash which is a no-op when the OS separator is '/'.
			Name:     strings.ReplaceAll(s.Path, "\\", "/"),
			Method:   zip.Deflate,
			Modified: zipEpoch,
		})
		if err != nil {
			return nil, fmt.Errorf("zip create %s: %w", s.Path, err)
		}
		if _, err := f.Write([]byte(s.Content)); err != nil {
			return nil, fmt.Errorf("zip write %s: %w", s.Path, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("zip close: %w", err)
	}
	return buf.Bytes(), nil
}
