package deploy

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widgrensit/asobi-cli/internal/client"
)

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
			Path:    rel,
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
		f, err := w.Create(s.Path)
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
