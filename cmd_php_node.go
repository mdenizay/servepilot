package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════
// PHP Management
// ═══════════════════════════════════════════════════════════════════════════

func cmdPHP(sub string) {
	switch sub {
	case "install":
		phpInstall()
	case "switch":
		phpSwitch()
	case "list":
		phpList()
	default:
		fmt.Printf("Unknown php subcommand: %s\n", sub)
	}
}

func phpInstall() {
	version := getFlag("--version")
	if version == "" && len(os.Args) > 3 {
		version = os.Args[3]
	}
	if version == "" {
		fmt.Println("Usage: servepilot php install --version <7.4|8.0|8.1|8.2|8.3|8.4>")
		os.Exit(1)
	}

	validVersions := map[string]bool{"7.4": true, "8.0": true, "8.1": true, "8.2": true, "8.3": true, "8.4": true}
	if !validVersions[version] {
		printError(fmt.Sprintf("Invalid PHP version: %s. Supported: 7.4, 8.0, 8.1, 8.2, 8.3, 8.4", version))
		os.Exit(1)
	}

	cfg, _ := loadServerConfig()

	// Check if already installed
	for _, v := range cfg.PHPVersions {
		if v == version {
			printInfo(fmt.Sprintf("PHP %s is already installed", version))
			return
		}
	}

	printStep(fmt.Sprintf("Installing PHP %s...", version))
	installPHPVersion(version)

	cfg.PHPVersions = append(cfg.PHPVersions, version)
	saveServerConfig(cfg)

	logAction("php", "install", fmt.Sprintf("version=%s", version))
	printSuccess(fmt.Sprintf("PHP %s installed with FPM", version))
}

func phpSwitch() {
	domain := getFlag("--domain")
	version := getFlag("--version")

	if domain == "" || version == "" {
		fmt.Println("Usage: servepilot php switch --domain <domain> --version <8.x>")
		os.Exit(1)
	}

	site, err := loadSiteConfig(domain)
	if err != nil {
		printError(fmt.Sprintf("Site not found: %s", domain))
		os.Exit(1)
	}

	if site.Type == "nextjs" || site.Type == "static" {
		printError("This site type doesn't use PHP")
		os.Exit(1)
	}

	// Check if PHP version is installed
	cfg, _ := loadServerConfig()
	found := false
	for _, v := range cfg.PHPVersions {
		if v == version {
			found = true
			break
		}
	}
	if !found {
		printError(fmt.Sprintf("PHP %s is not installed. Run 'servepilot php install --version %s' first", version, version))
		os.Exit(1)
	}

	oldVersion := site.PHPVersion
	site.PHPVersion = version

	// Regenerate Nginx config
	nginxConf := generateNginxConfig(site)
	confPath := filepath.Join(NGINX_SITES, domain+".conf")
	os.WriteFile(confPath, []byte(nginxConf), 0644)

	saveSiteConfig(site)
	reloadNginx()

	logAction(domain, "php_switch", fmt.Sprintf("%s -> %s", oldVersion, version))
	printSuccess(fmt.Sprintf("Switched %s from PHP %s to PHP %s", domain, oldVersion, version))
}

func phpList() {
	cfg, _ := loadServerConfig()

	fmt.Println("\n  ╭─────────────────────────────────────────────╮")
	fmt.Println("  │        Installed PHP Versions                │")
	fmt.Println("  ├─────────────────────────────────────────────┤")

	for _, v := range cfg.PHPVersions {
		status, _ := runCmd("systemctl", "is-active", fmt.Sprintf("php%s-fpm", v))
		icon := "🔴"
		if status == "active" {
			icon = "🟢"
		}
		fmt.Printf("  │  %s PHP %s (php%s-fpm: %s)\n", icon, v, v, status)
	}
	fmt.Println("  ╰─────────────────────────────────────────────╯")
}

// ═══════════════════════════════════════════════════════════════════════════
// Node.js Management
// ═══════════════════════════════════════════════════════════════════════════

func cmdNode(sub string) {
	switch sub {
	case "install":
		nodeInstall()
	case "switch":
		nodeSwitch()
	case "list":
		nodeList()
	default:
		fmt.Printf("Unknown node subcommand: %s\n", sub)
	}
}

func nodeInstall() {
	version := getFlag("--version")
	if version == "" && len(os.Args) > 3 {
		version = os.Args[3]
	}
	if version == "" {
		fmt.Println("Usage: servepilot node install --version <18|20|22>")
		os.Exit(1)
	}

	cfg, _ := loadServerConfig()

	for _, v := range cfg.NodeVersions {
		if v == version {
			printInfo(fmt.Sprintf("Node.js %s is already installed", version))
			return
		}
	}

	printStep(fmt.Sprintf("Installing Node.js %s...", version))
	installNodeVersion(version)

	cfg.NodeVersions = append(cfg.NodeVersions, version)
	saveServerConfig(cfg)

	logAction("node", "install", fmt.Sprintf("version=%s", version))
	printSuccess(fmt.Sprintf("Node.js %s installed", version))
}

func nodeSwitch() {
	version := getFlag("--version")
	if version == "" {
		fmt.Println("Usage: servepilot node switch --version <version>")
		os.Exit(1)
	}

	printStep(fmt.Sprintf("Switching default Node.js to %s...", version))
	_, err := runCmd("bash", "-c", fmt.Sprintf(`
		export NVM_DIR="/opt/nvm"
		[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
		nvm alias default %s
	`, version))

	if err != nil {
		printError(fmt.Sprintf("Failed to switch Node.js: %v", err))
		os.Exit(1)
	}

	logAction("node", "switch", fmt.Sprintf("version=%s", version))
	printSuccess(fmt.Sprintf("Default Node.js switched to %s", version))
}

func nodeList() {
	output, _ := runCmd("bash", "-c", `
		export NVM_DIR="/opt/nvm"
		[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
		nvm ls --no-colors 2>/dev/null | grep -v "N/A" | grep -v "^$"
	`)

	fmt.Println("\n  ╭─────────────────────────────────────────────╮")
	fmt.Println("  │        Installed Node.js Versions            │")
	fmt.Println("  ├─────────────────────────────────────────────┤")

	if strings.TrimSpace(output) == "" {
		fmt.Println("  │  No Node.js versions found                  │")
	} else {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				fmt.Printf("  │  🟢 %s\n", line)
			}
		}
	}
	fmt.Println("  ╰─────────────────────────────────────────────╯")
}
