package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widgrensit/asobi-go/internal/client"
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
