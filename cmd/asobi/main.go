package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/widgrensit/asobi-cli/internal/auth"
	"github.com/widgrensit/asobi-cli/internal/client"
	"github.com/widgrensit/asobi-cli/internal/config"
	"github.com/widgrensit/asobi-cli/internal/deploy"
)

const defaultSaasURL = "https://saas.asobi.dev"

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
	case "create":
		cmdCreate()
	case "deploy":
		cmdDeploy()
	case "stop":
		cmdStop()
	case "start":
		cmdStart()
	case "delete":
		cmdDelete()
	case "envs":
		cmdEnvs()
	case "destroy":
		cmdDestroy()
	case "env":
		cmdEnv()
	case "health":
		cmdHealth()
	case "config":
		cmdConfig()
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
  asobi create <name> [--size xs|s|m|l]  Create an environment
  asobi deploy <name> [dir]    Deploy Lua scripts to an environment
  asobi stop <name>            Stop an environment
  asobi start <name>           Start an environment
  asobi delete <name>          Delete an environment
  asobi envs                   List your environments
  asobi health                 Check engine health
  asobi config set <k> <v>     Set config (url, api_key, saas_url)
  asobi config show            Show current config
  asobi help                   Show this help

Login options:
  --saas-url <url>           SaaS URL (default: ` + defaultSaasURL + `)
  --token-name <name>        Name for this CLI session (default: hostname)`)
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
	fmt.Printf("  Tenant:      %s\n", creds.TenantID)
	fmt.Printf("  Game:        %s\n", creds.GameID)
	fmt.Printf("  Environment: %s\n", creds.EnvName)
	fmt.Printf("  Engine:      %s\n", creds.EngineURL)
	fmt.Printf("\nCredentials stored in ~/.asobi/credentials.json\n")
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
	fmt.Printf("  Tenant:      %s\n", creds.TenantID)
	fmt.Printf("  Game:        %s\n", creds.GameID)
	fmt.Printf("  Environment: %s\n", creds.EnvName)
	fmt.Printf("  Engine:      %s\n", creds.EngineURL)
	fmt.Printf("  Device:      %s\n", creds.DeviceFingerprint)
	if len(creds.AccessToken) > 8 {
		fmt.Printf("  Token:       %s...%s\n", creds.AccessToken[:4], creds.AccessToken[len(creds.AccessToken)-4:])
	}
}

// --- Deploy ---

func cmdDeploy() {
	if len(os.Args) < 3 {
		fatal("usage: asobi deploy <env-name> [dir]")
	}

	envName := os.Args[2]
	dir := "."
	if len(os.Args) >= 4 && !strings.HasPrefix(os.Args[3], "--") {
		dir = os.Args[3]
	}

	scripts, err := deploy.CollectScripts(dir)
	if err != nil {
		fatal("collect scripts: %v", err)
	}
	if len(scripts) == 0 {
		fatal("no .lua files found in %s", dir)
	}

	fmt.Printf("Deploying %d scripts to %s...\n", len(scripts), envName)
	for _, s := range scripts {
		fmt.Printf("  %s (%d bytes)\n", s.Path, len(s.Content))
	}

	bundle, err := deploy.ZipScripts(scripts)
	if err != nil {
		fatal("zip scripts: %v", err)
	}
	fmt.Printf("Bundle: %d bytes\n", len(bundle))

	creds := mustLoadCreds()
	result, err := auth.DeployBundle(creds, envName, bundle)
	if err != nil {
		fatal("deploy: %v", err)
	}

	gen, _ := result["generation"].(float64)
	sha, _ := result["sha256"].(string)
	fmt.Printf("\nDeployed! generation=%d sha256=%s\n", int(gen), sha[:12]+"...")
}

func resolveDeployCredentials() (engineURL, apiKey string) {
	creds, _ := auth.LoadCredentials()
	if creds != nil && creds.AccessToken != "" {
		fmt.Println("Minting ephemeral deploy key...")
		key, err := auth.MintKey(creds)
		if err != nil {
			fatal("mint deploy key: %v\nTry: asobi login", err)
		}
		return creds.EngineURL, key
	}

	cfg, err := config.Load()
	if err != nil {
		fatal("load config: %v", err)
	}
	if cfg.APIKey == "" {
		fatal("not logged in and no API key configured.\n\nRun: asobi login\n  or: asobi config set api_key <key>")
	}
	return cfg.URL, cfg.APIKey
}

// --- Health ---

func cmdHealth() {
	creds, _ := auth.LoadCredentials()
	var url string
	if creds != nil && creds.EngineURL != "" {
		url = creds.EngineURL
	} else {
		cfg, err := config.Load()
		if err != nil {
			fatal("load config: %v", err)
		}
		url = cfg.URL
	}

	c := client.New(&config.Config{URL: url})
	result, err := c.Health()
	if err != nil {
		fatal("health check failed: %v", err)
	}

	status, _ := result["status"].(string)
	if status == "ok" {
		fmt.Printf("Engine at %s is healthy.\n", url)
	} else {
		fmt.Printf("Engine at %s returned: %v\n", url, result)
	}
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

// --- Ephemeral deploy ---

func cmdDeployEphemeral(name string, jsonOut bool) {
	creds, err := auth.LoadCredentials()
	if err != nil || creds == nil || creds.AccessToken == "" {
		fatal("not logged in. Run: asobi login")
	}

	if !jsonOut {
		fmt.Println("Creating ephemeral environment (1h TTL)...")
	}
	resp, err := auth.EphemeralDeploy(creds, name)
	if err != nil {
		fatal("ephemeral-deploy: %v", err)
	}

	if jsonOut {
		out, _ := json.Marshal(map[string]any{
			"env_id":     resp.EnvID,
			"api_key":    resp.RawKey,
			"expires_in": resp.ExpiresIn,
		})
		fmt.Println(string(out))
		return
	}

	fmt.Printf("\n🦝 Ephemeral environment created!\n")
	fmt.Printf("  env_id:     %s\n", resp.EnvID)
	fmt.Printf("  api_key:    %s\n", resp.RawKey)
	fmt.Printf("  expires_in: %ds (~%dm)\n", resp.ExpiresIn, resp.ExpiresIn/60)
	fmt.Printf("\nTo destroy explicitly: asobi destroy %s\n", resp.EnvID)
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
	if len(os.Args) < 3 {
		fatal("usage: asobi create <name> [--size xs|s|m|l]")
	}
	name := os.Args[2]
	size := "xs"
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--size" && i+1 < len(os.Args) {
			size = os.Args[i+1]
			i++
		}
	}

	creds := mustLoadCreds()
	result, err := auth.CreateEnv(creds, name, size)
	if err != nil {
		fatal("create: %v", err)
	}
	fmt.Printf("Environment created: %s (size: %s)\n", name, size)
	if env, ok := result["environment"].(map[string]interface{}); ok {
		if id, ok := env["id"].(string); ok {
			fmt.Printf("  id: %s\n", id)
		}
	}
}

func cmdStop() {
	if len(os.Args) < 3 {
		fatal("usage: asobi stop <name>")
	}
	creds := mustLoadCreds()
	if err := auth.EnvAction(creds, os.Args[2], "stop"); err != nil {
		fatal("stop: %v", err)
	}
	fmt.Printf("Environment %s stopping\n", os.Args[2])
}

func cmdStart() {
	if len(os.Args) < 3 {
		fatal("usage: asobi start <name>")
	}
	creds := mustLoadCreds()
	if err := auth.EnvAction(creds, os.Args[2], "start"); err != nil {
		fatal("start: %v", err)
	}
	fmt.Printf("Environment %s starting\n", os.Args[2])
}

func cmdDelete() {
	if len(os.Args) < 3 {
		fatal("usage: asobi delete <name>")
	}
	creds := mustLoadCreds()
	if err := auth.DeleteEnv(creds, os.Args[2]); err != nil {
		fatal("delete: %v", err)
	}
	fmt.Printf("Environment %s deleted\n", os.Args[2])
}

func cmdEnvs() {
	creds := mustLoadCreds()
	envs, err := auth.ListEnvs2(creds)
	if err != nil {
		fatal("list: %v", err)
	}
	if len(envs) == 0 {
		fmt.Println("No environments. Create one with: asobi create <name>")
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

func mustLoadCreds() *auth.Credentials {
	creds, err := auth.LoadCredentials()
	if err != nil {
		fatal("not logged in. Run: asobi login")
	}
	return creds
}
