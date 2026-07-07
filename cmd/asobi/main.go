package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/widgrensit/asobi-cli/internal/auth"
	"github.com/widgrensit/asobi-cli/internal/client"
	"github.com/widgrensit/asobi-cli/internal/config"
	"github.com/widgrensit/asobi-cli/internal/deploy"
	"github.com/widgrensit/asobi-cli/internal/dev"
	"github.com/widgrensit/asobi-cli/internal/scaffold"
	"github.com/widgrensit/asobi-cli/internal/template"
)

const defaultSaasURL = "https://console.asobi.dev"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "login":
		cmdLogin()
	case "logout":
		cmdLogout()
	case "whoami":
		cmdWhoami()
	case "games":
		cmdGames()
	case "use":
		cmdUse()
	case "create":
		cmdCreate()
	case "deploy":
		cmdDeploy()
	case "stop":
		cmdStop()
	case "start":
		cmdStart()
	case "resize":
		cmdResize()
	case "delete":
		cmdDelete()
	case "envs":
		cmdEnvs()
	case "init":
		cmdInit()
	case "dev":
		cmdDev()
	case "destroy":
		cmdDestroy()
	case "env":
		cmdEnv()
	case "health":
		cmdHealth()
	case "config":
		cmdConfig()
	case "version", "--version", "-v":
		cmdVersion()
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`asobi - Asobi game backend CLI

Usage:
  asobi login                  Login via browser (device-code flow)
  asobi logout                 Clear stored credentials
  asobi whoami                 Show current credential info
  asobi init [dir]             Scaffold a starter Lua game
  asobi init [dir] --template <engine>
                              Scaffold a runnable demo (defold|godot|unity)
  asobi dev [--port N] [--dir <lua>]
                              Run a local backend for your Lua game (Docker)
  asobi games                  List your tenant's games
  asobi use <slug>             Set the active game
  asobi create <name> [--size xs|s|m|l] [--game <slug>]  Create an environment
  asobi deploy <name> [dir] [--game <slug>]    Deploy Lua scripts to an environment
  asobi stop <name> [--game <slug>]            Stop an environment
  asobi start <name> [--game <slug>]           Start an environment
  asobi resize <name> --size <xs|s|m|l> [--game <slug>]  Resize an environment
  asobi delete <name> [--game <slug>]          Delete an environment
  asobi envs [--game <slug>]   List your environments
  asobi health [env] [--game <slug>]   Check engine health (of an environment)
  asobi config set <k> <v>     Set config (url, api_key, saas_url)
  asobi config show            Show current config
  asobi version                Show version, commit, and build date
  asobi help                   Show this help

Game selection:
  Env operations resolve the game from --game, then the active game
  (asobi use <slug>). With no game set the CLI prompts when interactive.

Login options:
  --saas-url <url>           SaaS URL (default: ` + defaultSaasURL + `)
  --token-name <name>        Name for this CLI session (default: hostname)`)
}

// --- Version ---

func cmdVersion() {
	fmt.Printf("asobi %s\n", version)
	fmt.Printf("  commit: %s\n", commit)
	fmt.Printf("  built:  %s\n", date)
}

// --- Login/Logout/Whoami ---

func cmdLogin() {
	saasURL := defaultSaasURL
	tokenName := auth.DeviceFingerprint()

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--saas-url":
			if i+1 >= len(os.Args) {
				fatal("--saas-url requires a value")
			}
			i++
			saasURL = os.Args[i]
		case "--token-name":
			if i+1 >= len(os.Args) {
				fatal("--token-name requires a value")
			}
			i++
			tokenName = os.Args[i]
		default:
			fatal("unknown login flag: %s", os.Args[i])
		}
	}

	if err := config.RequireSecureURL(saasURL); err != nil {
		fatal("%v", err)
	}

	fmt.Printf("Connecting to %s\n", saasURL)
	fmt.Printf("Token name: %s\n", tokenName)

	creds, err := auth.Login(saasURL, tokenName)
	if err != nil {
		fatal("login failed: %v", err)
	}

	if err := auth.SaveCredentials(creds); err != nil {
		fatal("save credentials: %v", err)
	}

	fmt.Println("\nLogin successful!")
	fmt.Printf("  Tenant: %s\n", creds.TenantID)
	fmt.Printf("  Game:   %s\n", gameLabel(creds.ActiveGame))
	fmt.Printf("  SaaS:   %s\n", creds.SaasURL)
	fmt.Printf("\nCredentials stored in ~/.asobi/credentials.json\n")
	fmt.Println("Run `asobi games` to list games, then `asobi use <slug>` to pick one.")
	fmt.Println("Run `asobi create <name>` to create an environment.")
}

func cmdLogout() {
	if err := auth.DeleteCredentials(); err != nil {
		fatal("logout: %v", err)
	}
	fmt.Println("Logged out. Credentials removed.")
}

func cmdWhoami() {
	creds, err := auth.LoadCredentials()
	if err != nil {
		fatal("load credentials: %v", err)
	}
	if creds == nil {
		fmt.Println("Not logged in. Run: asobi login")
		return
	}
	fmt.Printf("Logged in via %s\n", creds.SaasURL)
	fmt.Printf("  Tenant: %s\n", creds.TenantID)
	fmt.Printf("  Game:   %s\n", gameLabel(creds.ActiveGame))
	fmt.Printf("  Device: %s\n", creds.DeviceFingerprint)
	if len(creds.AccessToken) > 8 {
		fmt.Printf("  Token:  %s...%s\n", creds.AccessToken[:4], creds.AccessToken[len(creds.AccessToken)-4:])
	}
}

// --- Deploy ---

func cmdDeploy() {
	gameFlag, args := extractFlag(os.Args[2:], "--game")
	if len(args) < 1 {
		fatal("usage: asobi deploy <env-name> [dir] [--game <slug>]")
	}

	envName := args[0]
	dir := "."
	if len(args) >= 2 && !strings.HasPrefix(args[1], "--") {
		dir = args[1]
	}

	scripts, err := deploy.CollectScripts(dir)
	if err != nil {
		fatal("collect scripts: %v", err)
	}
	if len(scripts) == 0 {
		fatal("no .lua files found in %s", dir)
	}

	creds := mustLoadCreds()
	game := resolveGame(gameFlag, creds)

	fmt.Printf("Deploying %d scripts to %s (game: %s)...\n", len(scripts), envName, game)
	for _, s := range scripts {
		fmt.Printf("  %s (%d bytes)\n", s.Path, len(s.Content))
	}

	bundle, err := deploy.ZipScripts(scripts)
	if err != nil {
		fatal("zip scripts: %v", err)
	}
	fmt.Printf("Bundle: %d bytes\n", len(bundle))

	result, err := auth.DeployBundle(creds, game, envName, bundle)
	if err != nil {
		fatal("deploy: %v", err)
	}

	gen, _ := result["generation"].(float64)
	sha, _ := result["sha256"].(string)
	fmt.Printf("\nDeployed to %s (game: %s)! generation=%d sha256=%s\n", envName, game, int(gen), sha[:12]+"...")
}

// --- Health ---

func cmdHealth() {
	creds, _ := auth.LoadCredentials()
	url := resolveHealthURL(creds, os.Args[2:])

	c := client.New(&config.Config{URL: url})
	result, err := c.Health()
	if err != nil {
		fatal("health check failed for %s: %v", url, err)
	}

	status, _ := result["status"].(string)
	if isHealthy(status) {
		fmt.Printf("Engine at %s is healthy.\n", url)
	} else {
		fmt.Printf("Engine at %s returned: %v\n", url, result)
	}
}

func isHealthy(status string) bool {
	return status == "ok" || status == "healthy"
}

// resolveHealthURL picks which engine to check:
//  1. an environment bound at login (creds.EngineURL),
//  2. otherwise the active game's environment (named, or the only one),
//  3. otherwise the configured URL (localhost default, for self-hosting).
func resolveHealthURL(creds *auth.Credentials, args []string) string {
	if creds != nil && creds.EngineURL != "" {
		return creds.EngineURL
	}
	gameFlag, rest := extractFlag(args, "--game")
	if creds != nil && (creds.ActiveGame != "" || gameFlag != "") {
		return resolveEnvEndpoint(creds, resolveGame(gameFlag, creds), firstArg(rest))
	}
	cfg, err := config.Load()
	if err != nil {
		fatal("load config: %v", err)
	}
	return cfg.URL
}

func resolveEnvEndpoint(creds *auth.Credentials, game, envName string) string {
	envs, err := auth.ListEnvs2(creds, game)
	if err != nil {
		fatal("resolve environment for %s: %v", game, err)
	}
	env, err := selectEnv(envs, envName)
	if err != nil {
		fatal("%v (run `asobi envs` to list, or pass `asobi health <env>`)", err)
	}
	endpoint, _ := env["endpoint_url"].(string)
	if endpoint == "" {
		name, _ := env["name"].(string)
		status, _ := env["provisioning_status"].(string)
		fatal("environment %s has no endpoint yet (status: %s)", name, status)
	}
	return endpoint
}

// selectEnv resolves one environment from a game's list: by name when given,
// otherwise the sole environment. It is ambiguous with several and no name.
func selectEnv(envs []map[string]interface{}, envName string) (map[string]interface{}, error) {
	if len(envs) == 0 {
		return nil, fmt.Errorf("no environments for this game")
	}
	if envName != "" {
		for _, e := range envs {
			if name, _ := e["name"].(string); name == envName {
				return e, nil
			}
		}
		return nil, fmt.Errorf("no environment named %q", envName)
	}
	if len(envs) == 1 {
		return envs[0], nil
	}
	return nil, fmt.Errorf("multiple environments - specify one")
}

// firstArg returns the first non-flag argument, or "".
func firstArg(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			return a
		}
	}
	return ""
}

// --- Config ---

func cmdConfig() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: asobi config <set|show>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "show":
		cfg, err := config.Load()
		if err != nil {
			fatal("load config: %v", err)
		}
		fmt.Printf("Engine URL: %s\n", cfg.URL)
		if cfg.APIKey != "" {
			fmt.Printf("API Key:    %s...%s\n", cfg.APIKey[:10], cfg.APIKey[len(cfg.APIKey)-4:])
		} else {
			fmt.Println("API Key:    (not set)")
		}
		fmt.Println("\nConfig is the manual fallback. Use `asobi login` for the recommended flow.")

	case "set":
		if len(os.Args) < 5 {
			fmt.Println("Usage: asobi config set <key> <value>")
			fmt.Println("Keys: url, api_key, saas_url")
			os.Exit(1)
		}
		key, value := os.Args[3], os.Args[4]
		cfg, err := config.Load()
		if err != nil {
			fatal("load config: %v", err)
		}
		switch key {
		case "url":
			if err := config.RequireSecureURL(value); err != nil {
				fatal("%v", err)
			}
			cfg.URL = value
		case "api_key":
			cfg.APIKey = value
		default:
			fatal("unknown config key: %s (valid: url, api_key)", key)
		}
		if err := config.Save(cfg); err != nil {
			fatal("save config: %v", err)
		}
		fmt.Printf("Set %s.\n", key)

	default:
		fmt.Println("Usage: asobi config <set|show>")
		os.Exit(1)
	}
}

func startSpinner() func() {
	frames := []string{
		"🦝 Deploying.  ",
		"🦝 Deploying.. ",
		"🦝 Deploying...",
	}
	var once sync.Once
	done := make(chan struct{})
	go func() {
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				fmt.Printf("\r%s", frames[i%len(frames)])
				i++
				time.Sleep(400 * time.Millisecond)
			}
		}
	}()
	return func() {
		once.Do(func() { close(done) })
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

// --- Destroy ---

func cmdDestroy() {
	if len(os.Args) < 3 {
		fatal("destroy requires an env_id\n\nUsage: asobi destroy <env_id>")
	}
	envID := os.Args[2]

	creds, err := auth.LoadCredentials()
	if err != nil || creds == nil || creds.AccessToken == "" {
		fatal("not logged in. Run: asobi login")
	}

	if err := auth.Destroy(creds, envID); err != nil {
		fatal("%v", err)
	}
	fmt.Printf("Destroyed %s\n", envID)
}

// --- Env ---

func cmdEnv() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: asobi env list [--ephemeral] [--json]")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "list":
		cmdEnvList()
	default:
		fatal("unknown env subcommand: %s", os.Args[2])
	}
}

func cmdEnvList() {
	ephemeral := false
	jsonOut := false
	for _, arg := range os.Args[3:] {
		switch arg {
		case "--ephemeral":
			ephemeral = true
		case "--json":
			jsonOut = true
		default:
			fatal("unknown env list flag: %s", arg)
		}
	}

	creds, err := auth.LoadCredentials()
	if err != nil || creds == nil || creds.AccessToken == "" {
		fatal("not logged in. Run: asobi login")
	}

	envs, err := auth.ListEnvs(creds, ephemeral)
	if err != nil {
		fatal("%v", err)
	}

	if jsonOut {
		out, _ := json.Marshal(envs)
		fmt.Println(string(out))
		return
	}

	if len(envs) == 0 {
		fmt.Println("No environments.")
		return
	}
	fmt.Printf("%-40s %-20s %-10s %-10s %s\n", "ID", "NAME", "STATUS", "EPHEMERAL", "EXPIRES")
	for _, e := range envs {
		eph := "no"
		if e.IsEphemeral {
			eph = "yes"
		}
		fmt.Printf("%-40s %-20s %-10s %-10s %s\n", e.ID, e.Name, e.Status, eph, e.ExpiresAt)
	}
}

// --- New environment commands ---

func cmdCreate() {
	gameFlag, args := extractFlag(os.Args[2:], "--game")
	sizeFlag, args := extractFlag(args, "--size")
	if len(args) < 1 {
		fatal("usage: asobi create <name> [--size xs|s|m|l] [--game <slug>]")
	}
	name := args[0]
	size := sizeFlag
	if size == "" {
		size = "xs"
	}

	creds := mustLoadCreds()
	game := resolveGame(gameFlag, creds)
	result, err := auth.CreateEnv(creds, game, name, size)
	if err != nil {
		fatal("create: %v", err)
	}
	fmt.Printf("Environment created: %s (game: %s, size: %s)\n", name, game, size)
	if env, ok := result["environment"].(map[string]interface{}); ok {
		if id, ok := env["id"].(string); ok {
			fmt.Printf("  id: %s\n", id)
		}
	}
}

func cmdStop() {
	gameFlag, args := extractFlag(os.Args[2:], "--game")
	if len(args) < 1 {
		fatal("usage: asobi stop <name> [--game <slug>]")
	}
	creds := mustLoadCreds()
	game := resolveGame(gameFlag, creds)
	if err := auth.EnvAction(creds, game, args[0], "stop"); err != nil {
		fatal("stop: %v", err)
	}
	fmt.Printf("Environment %s stopping\n", args[0])
}

func cmdStart() {
	gameFlag, args := extractFlag(os.Args[2:], "--game")
	if len(args) < 1 {
		fatal("usage: asobi start <name> [--game <slug>]")
	}
	creds := mustLoadCreds()
	game := resolveGame(gameFlag, creds)
	if err := auth.EnvAction(creds, game, args[0], "start"); err != nil {
		fatal("start: %v", err)
	}
	fmt.Printf("Environment %s starting\n", args[0])
}

func cmdResize() {
	gameFlag, args := extractFlag(os.Args[2:], "--game")
	sizeFlag, args := extractFlag(args, "--size")
	if len(args) < 1 {
		fatal("usage: asobi resize <name> --size <xs|s|m|l> [--game <slug>]")
	}
	if sizeFlag == "" {
		fatal("resize requires --size <xs|s|m|l>")
	}
	creds := mustLoadCreds()
	game := resolveGame(gameFlag, creds)
	if err := auth.ResizeEnv(creds, game, args[0], sizeFlag); err != nil {
		fatal("resize: %v", err)
	}
	fmt.Printf("Environment %s resizing to %s\n", args[0], sizeFlag)
}

func cmdDelete() {
	gameFlag, args := extractFlag(os.Args[2:], "--game")
	if len(args) < 1 {
		fatal("usage: asobi delete <name> [--game <slug>]")
	}
	creds := mustLoadCreds()
	game := resolveGame(gameFlag, creds)
	if err := auth.DeleteEnv(creds, game, args[0]); err != nil {
		fatal("delete: %v", err)
	}
	fmt.Printf("Environment %s deleted\n", args[0])
}

func cmdEnvs() {
	gameFlag, _ := extractFlag(os.Args[2:], "--game")
	creds := mustLoadCreds()
	game := resolveGame(gameFlag, creds)
	envs, err := auth.ListEnvs2(creds, game)
	if err != nil {
		fatal("list: %v", err)
	}
	if len(envs) == 0 {
		fmt.Printf("No environments for %s. Create one with: asobi create <name> --game %s\n", game, game)
		return
	}
	fmt.Printf("%-20s %-6s %-15s %s\n", "NAME", "SIZE", "STATUS", "ENDPOINT")
	for _, e := range envs {
		name, _ := e["name"].(string)
		size, _ := e["size"].(string)
		if size == "" {
			size, _ = e["resource_tier"].(string)
		}
		status, _ := e["provisioning_status"].(string)
		endpoint, _ := e["endpoint_url"].(string)
		if endpoint == "" {
			endpoint = "-"
		}
		fmt.Printf("%-20s %-6s %-15s %s\n", name, strings.ToUpper(size), status, endpoint)
	}
}

// --- Games ---

func cmdGames() {
	creds := mustLoadCreds()
	games, err := auth.ListGames(creds)
	if err != nil {
		fatal("games: %v", err)
	}
	if len(games) == 0 {
		fmt.Println("No games in this tenant.")
		return
	}
	fmt.Printf("%-2s %-24s %s\n", "", "SLUG", "NAME")
	for _, g := range games {
		marker := "  "
		if g.Slug == creds.ActiveGame {
			marker = "* "
		}
		fmt.Printf("%-2s %-24s %s\n", marker, g.Slug, g.Name)
	}
	if creds.ActiveGame == "" {
		fmt.Println("\nNo active game. Run `asobi use <slug>` to select one.")
	}
}

func cmdUse() {
	if len(os.Args) < 3 {
		fatal("usage: asobi use <slug>")
	}
	slug := os.Args[2]
	creds := mustLoadCreds()
	games, err := auth.ListGames(creds)
	if err != nil {
		fatal("use: %v", err)
	}
	found := false
	for _, g := range games {
		if g.Slug == slug {
			found = true
			break
		}
	}
	if !found {
		fatal("unknown game %q. Run `asobi games` to see your games.", slug)
	}
	creds.ActiveGame = slug
	if err := auth.SaveCredentials(creds); err != nil {
		fatal("save credentials: %v", err)
	}
	fmt.Printf("Active game set to %s\n", slug)
}

// --- Init ---

func cmdInit() {
	args := os.Args[2:]
	engine, args := extractFlag(args, "--template")

	dir := "."
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			dir = a
			break
		}
	}
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatal("create %s: %v", dir, err)
		}
	}

	if engine != "" {
		t, ok := template.Get(engine)
		if !ok {
			fatal("unknown template %q; available: %s", engine, strings.Join(template.Engines(), ", "))
		}
		fmt.Printf("Fetching the %s template into %s ...\n", t.Name, dir)
		created, err := template.Fetch(engine, dir)
		if err != nil {
			fatal("init: %v", err)
		}
		fmt.Printf("Scaffolded the %s game template in %s\n", t.Name, dir)
		for _, f := range created {
			fmt.Printf("  created %s\n", f)
		}
		fmt.Println("\nNext steps:")
		n := 1
		step := func(s string) { fmt.Printf("  %d. %s\n", n, s); n++ }
		if dir != "." {
			step("cd " + dir)
		}
		step("Open README.md - it walks through starting the backend and opening the project in " + t.Name + ".")
		return
	}

	created, err := scaffold.Init(dir)
	if err != nil {
		fatal("init: %v", err)
	}

	fmt.Printf("Scaffolded a starter Asobi game in %s\n", dir)
	for _, f := range created {
		fmt.Printf("  created %s\n", f)
	}
	fmt.Println("\nNext steps:")
	n := 1
	step := func(s string) { fmt.Printf("  %d. %s\n", n, s); n++ }
	if dir != "." {
		step("cd " + dir)
	}
	step("asobi login")
	step("asobi use <game>       (list yours: asobi games)")
	step("asobi create <env>     (e.g. asobi create prod)")
	step("asobi deploy <env> lua")
	step("Connect a client - Defold quickstart: https://github.com/widgrensit/asobi-defold")
}

func cmdDev() {
	args := os.Args[2:]
	portStr, args := extractFlag(args, "--port")
	luaDir, _ := extractFlag(args, "--dir")

	port := 8084
	if portStr != "" {
		n, err := strconv.Atoi(portStr)
		if err != nil || n < 1 || n > 65535 {
			fatal("--port must be a number 1-65535, got %q", portStr)
		}
		port = n
	}

	root, err := os.Getwd()
	if err != nil {
		fatal("dev: %v", err)
	}
	if err := dev.Run(dev.Options{Root: root, LuaOverride: luaDir, Port: port}); err != nil {
		fatal("%v", err)
	}
}

// --- Game resolution helpers ---

// extractFlag pulls a "--name value" pair out of args, returning the value
// (empty if absent) and the remaining positional/other args.
func extractFlag(args []string, name string) (string, []string) {
	var val string
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == name {
			if i+1 < len(args) {
				val = args[i+1]
				i++
			}
			continue
		}
		rest = append(rest, args[i])
	}
	return val, rest
}

// resolveGame determines the effective game for an env operation:
// explicit --game, then the persisted active game, then (interactively) a
// picker or auto-selection, else a clear error.
func resolveGame(explicit string, creds *auth.Credentials) string {
	if explicit != "" {
		return explicit
	}
	if creds.ActiveGame != "" {
		return creds.ActiveGame
	}
	games, err := auth.ListGames(creds)
	if err != nil {
		fatal("No game selected. Run `asobi games` then `asobi use <slug>`, or pass --game <slug>.\n(could not list games: %v)", err)
	}
	switch {
	case len(games) == 1:
		return games[0].Slug
	case len(games) >= 2 && stdinIsTTY():
		return pickGame(games)
	default:
		fatal("No game selected. Run `asobi games` then `asobi use <slug>`, or pass --game <slug>.")
	}
	return ""
}

func stdinIsTTY() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func pickGame(games []auth.Game) string {
	fmt.Fprintln(os.Stderr, "Select a game:")
	for i, g := range games {
		fmt.Fprintf(os.Stderr, "  %d) %s  %s\n", i+1, g.Slug, g.Name)
	}
	fmt.Fprintf(os.Stderr, "Select a game [1-%d]: ", len(games))
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(games) {
		fatal("invalid selection %q", strings.TrimSpace(line))
	}
	return games[choice-1].Slug
}

func gameLabel(slug string) string {
	if slug == "" {
		return "none - run asobi use <slug>"
	}
	return slug
}

func mustLoadCreds() *auth.Credentials {
	creds, err := auth.LoadCredentials()
	if err != nil || creds == nil || creds.AccessToken == "" {
		fatal("not logged in. Run: asobi login")
	}
	return creds
}
