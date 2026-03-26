package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdUpdate() {
	// --set-source just saves the repo path, used by setup.sh
	if src := getFlag("--set-source"); src != "" {
		cfg := loadServerConfig()
		cfg.RepoPath = src
		cfg.InstalledVersion = VERSION
		saveServerConfig(cfg)
		printSuccess(fmt.Sprintf("Source path saved: %s", src))
		return
	}

	cfg := loadServerConfig()

	repoPath := cfg.RepoPath
	if repoPath == "" {
		// Try common locations
		home, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(home, "servepilot"),
			"/root/servepilot",
			"/opt/servepilot-src",
		}
		for _, c := range candidates {
			if fileExists(filepath.Join(c, "main.go")) {
				repoPath = c
				break
			}
		}
	}

	if repoPath == "" || !fileExists(filepath.Join(repoPath, "main.go")) {
		printError("Source directory not found.")
		fmt.Println("Run once from your install directory:")
		fmt.Println("  servepilot update --set-source /path/to/servepilot")
		os.Exit(1)
	}

	fmt.Printf("\n  ServePilot Update\n")
	fmt.Printf("  Source: %s\n", repoPath)
	fmt.Printf("  Current version: v%s\n\n", VERSION)

	// ── 1. git pull ───────────────────────────────────────────────────────────
	printStep("Pulling latest changes...")
	out, err := runCmd("git", "-C", repoPath, "pull", "--ff-only")
	if err != nil {
		printError("git pull failed: " + strings.TrimSpace(out))
		os.Exit(1)
	}
	if strings.Contains(out, "Already up to date") {
		printInfo("Already up to date.")
	} else {
		printSuccess("Repository updated")
		fmt.Println(strings.TrimSpace(out))
	}

	// ── 2. go mod tidy ───────────────────────────────────────────────────────
	printStep("Updating dependencies...")
	if _, err := runCmdInDir(repoPath, "go", "mod", "tidy"); err != nil {
		printError("go mod tidy failed")
		os.Exit(1)
	}

	// ── 3. go build ──────────────────────────────────────────────────────────
	printStep("Building binary...")
	if _, err := runCmdInDir(repoPath, "go", "build",
		"-ldflags=-s -w", "-o", "/usr/local/bin/servepilot", "."); err != nil {
		printError("Build failed")
		os.Exit(1)
	}
	printSuccess("Binary updated: /usr/local/bin/servepilot")

	// ── 4. Regenerate deploy.sh ───────────────────────────────────────────────
	printStep("Regenerating deploy scripts...")
	setupWebhookListener()
	printSuccess("Deploy scripts updated")

	// ── 5. Restart panel services ─────────────────────────────────────────────
	printStep("Restarting panel services...")
	runCmdSilent("systemctl", "restart", "servepilot-panel-api")
	runCmdSilent("systemctl", "restart", "servepilot-panel-ui")

	// ── 6. Save new version to config ─────────────────────────────────────────
	cfg = loadServerConfig()
	cfg.RepoPath = repoPath
	cfg.InstalledVersion = VERSION
	saveServerConfig(cfg)

	printSuccess(fmt.Sprintf("ServePilot updated to v%s", VERSION))
}

// runCmdInDir runs a command with the given working directory.
func runCmdInDir(dir string, name string, args ...string) (string, error) {
	return runCmd("bash", "-c", fmt.Sprintf("cd %q && %s %s",
		dir, name, strings.Join(args, " ")))
}
