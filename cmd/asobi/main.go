package main

import (
	"fmt"
	"os"

	"github.com/widgrensit/asobi-go/internal/client"
	"github.com/widgrensit/asobi-go/internal/config"
	"github.com/widgrensit/asobi-go/internal/deploy"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
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
  asobi deploy <dir>       Deploy Lua scripts to the engine
  asobi health             Check engine health
  asobi config set <key> <value>  Set config (url, api_key)
  asobi config show        Show current config
  asobi help               Show this help`)
}

func cmdDeploy() {
	dir := "."
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}

	cfg, err := config.Load()
	if err != nil {
		fatal("load config: %v", err)
	}
	if cfg.APIKey == "" {
		fatal("no API key configured. Run: asobi config set api_key <key>")
	}

	scripts, err := deploy.CollectScripts(dir)
	if err != nil {
		fatal("collect scripts: %v", err)
	}
	if len(scripts) == 0 {
		fatal("no .lua files found in %s", dir)
	}

	fmt.Printf("Deploying %d scripts to %s...\n", len(scripts), cfg.URL)
	for _, s := range scripts {
		fmt.Printf("  %s (%d bytes)\n", s.Path, len(s.Content))
	}

	c := client.New(cfg)
	result, err := c.Deploy(scripts)
	if err != nil {
		fatal("%v", err)
	}

	fmt.Printf("\nDeployed %d scripts.\n", result.Deployed)
}

func cmdHealth() {
	cfg, err := config.Load()
	if err != nil {
		fatal("load config: %v", err)
	}

	c := client.New(cfg)
	result, err := c.Health()
	if err != nil {
		fatal("health check failed: %v", err)
	}

	status, _ := result["status"].(string)
	if status == "ok" {
		fmt.Printf("Engine at %s is healthy.\n", cfg.URL)
	} else {
		fmt.Printf("Engine at %s returned: %v\n", cfg.URL, result)
	}
}

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
		fmt.Printf("URL:     %s\n", cfg.URL)
		if cfg.APIKey != "" {
			fmt.Printf("API Key: %s...%s\n", cfg.APIKey[:10], cfg.APIKey[len(cfg.APIKey)-4:])
		} else {
			fmt.Println("API Key: (not set)")
		}

	case "set":
		if len(os.Args) < 5 {
			fmt.Println("Usage: asobi config set <key> <value>")
			fmt.Println("Keys: url, api_key")
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
