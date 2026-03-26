package main

import (
	"fmt"
	"os"
)

const VERSION = "1.1.0"

func main() {
	if os.Geteuid() != 0 {
		fmt.Println("⚠  ServePilot requires root privileges. Please run with sudo.")
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "init":
		cmdInit()
	case "site":
		if len(os.Args) < 3 {
			fmt.Println("Usage: servepilot site <add|remove|list|info> [options]")
			os.Exit(1)
		}
		cmdSite(os.Args[2])
	case "db":
		if len(os.Args) < 3 {
			fmt.Println("Usage: servepilot db <create|remove|list> [options]")
			os.Exit(1)
		}
		cmdDB(os.Args[2])
	case "php":
		if len(os.Args) < 3 {
			fmt.Println("Usage: servepilot php <install|switch|list> [options]")
			os.Exit(1)
		}
		cmdPHP(os.Args[2])
	case "node":
		if len(os.Args) < 3 {
			fmt.Println("Usage: servepilot node <install|switch|list> [options]")
			os.Exit(1)
		}
		cmdNode(os.Args[2])
	case "ssl":
		if len(os.Args) < 3 {
			fmt.Println("Usage: servepilot ssl <issue|renew|renew-all> [options]")
			os.Exit(1)
		}
		cmdSSL(os.Args[2])
	case "deploy":
		if len(os.Args) < 3 {
			fmt.Println("Usage: servepilot deploy <setup|trigger|log> [options]")
			os.Exit(1)
		}
		cmdDeploy(os.Args[2])
	case "secure":
		cmdSecure()
	case "backup":
		if len(os.Args) < 3 {
			fmt.Println("Usage: servepilot backup <create|restore|list> [options]")
			os.Exit(1)
		}
		cmdBackup(os.Args[2])
	case "panel":
		if len(os.Args) < 3 {
			fmt.Println("Usage: servepilot panel <setup|start|nginx> [options]")
			os.Exit(1)
		}
		cmdPanel(os.Args[2])
	case "status":
		cmdStatus()
	case "update":
		cmdUpdate()
	case "help", "--help", "-h":
		printUsage()
	case "version", "--version", "-v":
		fmt.Printf("ServePilot v%s\n", VERSION)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`
╔═══════════════════════════════════════════════════════════════╗
║                    ServePilot v%s                           ║
║           Self-Hosted Server Management CLI                   ║
║              Free & Open Source Forge Alternative              ║
╚═══════════════════════════════════════════════════════════════╝

USAGE:
    servepilot <command> [subcommand] [options]

COMMANDS:
    init                    Initialize server (install Nginx, PHP, Node, DB, security)
    site <sub>              Manage sites/projects
        add                 Add a new site (Laravel, Next.js, static, etc.)
        remove              Remove a site
        list                List all managed sites
        info                Show detailed site info
    db <sub>                Manage databases
        create              Create a new MySQL/PostgreSQL database + user
        remove              Remove a database
        list                List all databases
    php <sub>               Manage PHP versions
        install             Install a PHP version (7.4, 8.0, 8.1, 8.2, 8.3, 8.4)
        switch              Switch a site's PHP version
        list                List installed PHP versions
    node <sub>              Manage Node.js versions
        install             Install a Node.js version via nvm
        switch              Switch default Node.js version
        list                List installed Node.js versions
    ssl <sub>               Manage SSL certificates
        issue               Issue Let's Encrypt certificate for a site
        renew               Renew a specific certificate
        renew-all           Renew all certificates
    deploy <sub>            Git deployment management
        setup               Setup deploy key + webhook for a site
        trigger             Manually trigger deployment
        log                 View deployment logs
    secure                  Harden server security (firewall, fail2ban, SSH)
    backup <sub>            Backup management
        create              Create full backup (sites + databases)
        restore             Restore from backup
        list                List available backups
    status                  Show server & services status
    update                  Pull latest version and rebuild

EXAMPLES:
    servepilot init
    servepilot site add --domain example.com --type laravel --php 8.3
    servepilot site add --domain app.example.com --type nextjs --node 20
    servepilot db create --name myapp --user myapp_user
    servepilot ssl issue --domain example.com
    servepilot deploy setup --domain example.com --repo git@github.com:user/repo.git
    servepilot secure
    servepilot backup create

`, VERSION)
}
