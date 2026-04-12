package main

import (
	"fmt"
	"os"

	"github.com/widgrensit/asobi-cli/internal/auth"
	"github.com/widgrensit/asobi-cli/internal/client"
	"github.com/widgrensit/asobi-cli/internal/config"
	"github.com/widgrensit/asobi-cli/internal/deploy"
)

const defaultSaasURL = "https://app.asobi.dev"

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
	case "deploy":
		cmdDeploy()
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
  asobi login                Login via browser (device-code flow)
  asobi logout               Clear stored credentials
  asobi whoami               Show current credential info
  asobi deploy <dir>         Deploy Lua scripts to the engine
  asobi health               Check engine health
  asobi config set <k> <v>   Set config (url, api_key, saas_url)
  asobi config show          Show current config
  asobi help                 Show this help

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
	dir := "."
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}

	engineURL, apiKey := resolveDeployCredentials()

	scripts, err := deploy.CollectScripts(dir)
	if err != nil {
		fatal("collect scripts: %v", err)
	}
	if len(scripts) == 0 {
		fatal("no .lua files found in %s", dir)
	}

	fmt.Printf("Deploying %d scripts to %s...\n", len(scripts), engineURL)
	for _, s := range scripts {
		fmt.Printf("  %s (%d bytes)\n", s.Path, len(s.Content))
	}

	cfg := &config.Config{URL: engineURL, APIKey: apiKey}
	c := client.New(cfg)
	result, err := c.Deploy(scripts)
	if err != nil {
		fatal("%v", err)
	}

	fmt.Printf("\nDeployed %d scripts.\n", result.Deployed)
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

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
