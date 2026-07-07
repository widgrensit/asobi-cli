// Package dev runs a local Asobi backend for iterating on a Lua game. It is a
// thin orchestrator over Docker Compose: it writes a managed compose file and
// shells out to `docker compose up`. It is the no-account, no-key self-host side
// of the CLI - it never touches credentials or the control plane. Only cmdDev
// imports this package, so the control-plane commands keep zero Docker dependency.
package dev

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

// Options configures a dev run.
type Options struct {
	Root        string // project directory (usually the working dir)
	LuaOverride string // explicit Lua dir, from --dir; empty to auto-detect
	Port        int    // host + container port for the API/WebSocket
}

// Run resolves the Lua directory, checks Docker, writes a managed compose file,
// and runs `docker compose up` in the foreground until the user stops it.
func Run(opts Options) error {
	luaDir, err := resolveLuaDir(opts.Root, opts.LuaOverride)
	if err != nil {
		return err
	}
	if err := ensureDocker(); err != nil {
		return err
	}

	absLua, err := filepath.Abs(filepath.Join(opts.Root, luaDir))
	if err != nil {
		return err
	}
	composePath := filepath.Join(opts.Root, ".asobi", "dev-compose.yml")
	if err := os.MkdirAll(filepath.Dir(composePath), 0o755); err != nil {
		return fmt.Errorf("create .asobi: %w", err)
	}
	if err := os.WriteFile(composePath, []byte(renderCompose(absLua, opts.Port)), 0o644); err != nil {
		return fmt.Errorf("write compose: %w", err)
	}

	banner(luaDir, opts.Port, composePath)
	return composeUp(composePath, opts.Root)
}

// resolveLuaDir finds the Lua game directory: an explicit override, then lua/,
// then game/. It errors if none is a directory.
func resolveLuaDir(root, override string) (string, error) {
	for _, c := range []string{override, "lua", "game"} {
		if c == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(root, c)); err == nil && fi.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("no Lua game found (looked for lua/ or game/). Run `asobi init` first, or pass --dir <path>")
}

// ensureDocker verifies Docker and Compose v2 are installed and the daemon is
// up, returning an actionable message otherwise.
func ensureDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("`asobi dev` needs Docker. Install Docker Desktop (macOS/Windows) or Docker Engine (Linux): https://docs.docker.com/get-docker/")
	}
	if out, err := exec.Command("docker", "compose", "version").CombinedOutput(); err != nil {
		return fmt.Errorf("`asobi dev` needs Docker Compose v2 (`docker compose`): %s", strings.TrimSpace(string(out)))
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		return fmt.Errorf("the Docker daemon isn't running - start Docker Desktop (or the docker service) and try again")
	}
	return nil
}

func banner(luaDir string, port int, composePath string) {
	fmt.Println("Starting your Asobi dev server - no account, no keys, all local.")
	fmt.Printf("  Lua game:  ./%s\n", luaDir)
	fmt.Printf("  API + WS:  http://localhost:%d   (WebSocket at /ws)\n", port)
	fmt.Printf("  Compose:   %s (managed by asobi dev)\n", composePath)
	fmt.Println()
	fmt.Println("First boot pulls the asobi_lua image and runs migrations - give it a few seconds.")
	fmt.Println("Edit files under your Lua directory and they hot-reload; no restart needed.")
	fmt.Println("Connect a client or register a player: https://asobi.dev/docs/quickstart")
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()
}

// composeUp runs `docker compose up` in the foreground, streaming its logs. It
// suppresses the parent's default Ctrl+C handler so Compose can shut its
// containers down gracefully before this returns.
func composeUp(composePath, root string) error {
	cmd := exec.Command("docker", "compose", "-p", "asobi-dev", "-f", composePath, "up")
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	go func() {
		for range sig {
		}
	}()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up: %w", err)
	}
	return nil
}

func renderCompose(absLuaDir string, port int) string {
	return fmt.Sprintf(`# Managed by `+"`asobi dev`"+` - regenerated each run, safe to delete.
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: asobi_dev
    volumes:
      - asobi_dev_pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d asobi_dev -h 127.0.0.1 -p 5432"]
      interval: 3s
      timeout: 5s
      retries: 15
      start_period: 20s

  asobi:
    image: ghcr.io/widgrensit/asobi_lua:latest
    restart: on-failure
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "%d:%d"
    volumes:
      - "%s:/app/game:ro"
    environment:
      ASOBI_PORT: "%d"
      ASOBI_NODE_HOST: "127.0.0.1"
      ERLANG_COOKIE: asobi_dev
      ASOBI_DB_HOST: postgres
      ASOBI_DB_NAME: asobi_dev
      ASOBI_DB_USER: postgres
      ASOBI_DB_PASSWORD: postgres

volumes:
  asobi_dev_pgdata:
`, port, port, filepath.ToSlash(absLuaDir), port)
}
