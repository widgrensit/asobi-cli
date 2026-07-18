// Package scaffold writes a starter server-Lua game into a directory from one of
// a few genre templates (a real-time arena, a chat room, a turn-based game, a
// persistent world, or a plain default). The templates are embedded, so a
// scaffold is offline and instant - unlike the engine demos in package template,
// which fetch a full client+backend repo.
package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed all:templates
var templates embed.FS

// defaultGenre is scaffolded when Init is called with an empty genre (plain
// `asobi init` with no --template).
const defaultGenre = "basic"

// Genres returns the selectable genre names, sorted (e.g. arena, basic, chat,
// turn-based, world).
func Genres() []string {
	entries, err := templates.ReadDir("templates")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// IsGenre reports whether name is a known genre template.
func IsGenre(name string) bool {
	_, err := fs.Stat(templates, "templates/"+name)
	return err == nil
}

// Init scaffolds the genre's starter game into dir. An empty genre means the
// default starter. It returns the created file paths (relative to dir) and
// refuses to overwrite any existing target file, in particular lua/match.lua.
func Init(dir, genre string) ([]string, error) {
	if genre == "" {
		genre = defaultGenre
	}
	root := "templates/" + genre
	if !IsGenre(genre) {
		return nil, fmt.Errorf("unknown template %q; available: %s", genre, strings.Join(Genres(), ", "))
	}

	type file struct{ rel, content string }
	var files []file
	err := fs.WalkDir(templates, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		content, err := templates.ReadFile(p)
		if err != nil {
			return err
		}
		rel := filepath.FromSlash(strings.TrimPrefix(p, root+"/"))
		files = append(files, file{rel, string(content)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read %s template: %w", genre, err)
	}

	for _, f := range files {
		full := filepath.Join(dir, f.rel)
		if _, err := os.Stat(full); err == nil {
			return nil, fmt.Errorf("%s already exists; refusing to overwrite", full)
		}
	}

	var created []string
	for _, f := range files {
		full := filepath.Join(dir, f.rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return created, fmt.Errorf("create dir for %s: %w", f.rel, err)
		}
		if err := os.WriteFile(full, []byte(f.content), 0o644); err != nil {
			return created, fmt.Errorf("write %s: %w", f.rel, err)
		}
		created = append(created, f.rel)
	}
	sort.Strings(created)
	return created, nil
}
