package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

func cmdDB(sub string) {
	switch sub {
	case "create":
		dbCreate()
	case "remove":
		dbRemove()
	case "list":
		dbList()
	default:
		fmt.Printf("Unknown db subcommand: %s\n", sub)
	}
}

func generatePassword(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// ─── DB Create ───────────────────────────────────────────────────────────────

func dbCreate() {
	name := getFlag("--name")
	user := getFlag("--user")
	domain := getFlag("--domain")

	if name == "" {
		fmt.Println("Usage: servepilot db create --name <dbname> [--user <username>] [--domain <domain>]")
		os.Exit(1)
	}

	if user == "" {
		user = name + "_user"
	}

	cfg, _ := loadServerConfig()
	password := generatePassword(24)

	printStep(fmt.Sprintf("Creating database '%s' with user '%s'...", name, user))

	var err error
	if cfg.DBEngine == "postgresql" {
		err = createPostgresDB(name, user, password)
	} else {
		err = createMySQLDB(name, user, password)
	}

	if err != nil {
		printError(fmt.Sprintf("Failed to create database: %v", err))
		os.Exit(1)
	}

	// If domain is specified, update site config
	if domain != "" {
		site, siteErr := loadSiteConfig(domain)
		if siteErr == nil {
			site.Database = name
			saveSiteConfig(site)

			// If Laravel, update .env
			if site.Type == "laravel" {
				updateLaravelEnv(site.WebRoot, cfg.DBEngine, name, user, password)
			}
		}
	}

	logAction(name, "db_create", fmt.Sprintf("user=%s engine=%s", user, cfg.DBEngine))

	fmt.Printf(`
  ╭──────────────────────────────────────────────────────╮
  │  ✅ Database Created                                  │
  │  ─────────────────────────────────────                │
  │  Engine:    %s
  │  Database:  %s
  │  Username:  %s
  │  Password:  %s
  │  Host:      127.0.0.1
  │                                                        │
  │  ⚠  Save this password! It won't be shown again.      │
  ╰──────────────────────────────────────────────────────╯
`, cfg.DBEngine, name, user, password)
}

func createMySQLDB(name, user, password string) error {
	commands := fmt.Sprintf(`
		CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
		CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';
		GRANT ALL PRIVILEGES ON %s.* TO '%s'@'localhost';
		FLUSH PRIVILEGES;
	`, name, user, password, name, user)

	_, err := runCmd("mysql", "-e", commands)
	return err
}

func createPostgresDB(name, user, password string) error {
	// Create user
	_, err := runCmd("sudo", "-u", "postgres", "psql", "-c",
		fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s';", user, password))
	if err != nil {
		return err
	}

	// Create database
	_, err = runCmd("sudo", "-u", "postgres", "psql", "-c",
		fmt.Sprintf("CREATE DATABASE %s OWNER %s;", name, user))
	if err != nil {
		return err
	}

	// Grant privileges
	_, err = runCmd("sudo", "-u", "postgres", "psql", "-c",
		fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s;", name, user))
	return err
}

func updateLaravelEnv(webRoot, engine, dbName, dbUser, dbPass string) {
	envPath := webRoot + "/.env"
	if !fileExists(envPath) {
		// Create basic .env
		connection := "mysql"
		port := "3306"
		if engine == "postgresql" {
			connection = "pgsql"
			port = "5432"
		}
		env := fmt.Sprintf(`APP_NAME=Laravel
APP_ENV=production
APP_KEY=
APP_DEBUG=false
APP_URL=https://

DB_CONNECTION=%s
DB_HOST=127.0.0.1
DB_PORT=%s
DB_DATABASE=%s
DB_USERNAME=%s
DB_PASSWORD=%s

CACHE_DRIVER=redis
SESSION_DRIVER=redis
QUEUE_CONNECTION=redis

REDIS_HOST=127.0.0.1
REDIS_PASSWORD=ServePilotRedis2024
REDIS_PORT=6379
`, connection, port, dbName, dbUser, dbPass)
		os.WriteFile(envPath, []byte(env), 0644)
		runCmd("chown", "www-data:www-data", envPath)
		runCmd("chmod", "600", envPath)
	}
}

// ─── DB Remove ───────────────────────────────────────────────────────────────

func dbRemove() {
	name := getFlag("--name")
	user := getFlag("--user")
	confirm := getFlag("--confirm")

	if name == "" {
		fmt.Println("Usage: servepilot db remove --name <dbname> [--user <username>] --confirm yes")
		os.Exit(1)
	}
	if confirm != "yes" {
		fmt.Printf("⚠  This will permanently delete database '%s'!\n", name)
		fmt.Println("   Add --confirm yes to proceed.")
		return
	}
	if user == "" {
		user = name + "_user"
	}

	cfg, _ := loadServerConfig()

	if cfg.DBEngine == "postgresql" {
		runCmd("sudo", "-u", "postgres", "psql", "-c", fmt.Sprintf("DROP DATABASE IF EXISTS %s;", name))
		runCmd("sudo", "-u", "postgres", "psql", "-c", fmt.Sprintf("DROP USER IF EXISTS %s;", name))
	} else {
		runCmd("mysql", "-e", fmt.Sprintf("DROP DATABASE IF EXISTS %s; DROP USER IF EXISTS '%s'@'localhost'; FLUSH PRIVILEGES;", name, user))
	}

	logAction(name, "db_remove", "Database removed")
	printSuccess(fmt.Sprintf("Database '%s' and user '%s' removed", name, user))
}

// ─── DB List ─────────────────────────────────────────────────────────────────

func dbList() {
	cfg, _ := loadServerConfig()

	fmt.Println("\n  ╭─────────────────────────────────────────────╮")
	fmt.Printf("  │        Databases (%s)                      \n", cfg.DBEngine)
	fmt.Println("  ├─────────────────────────────────────────────┤")

	var output string
	if cfg.DBEngine == "postgresql" {
		output, _ = runCmd("sudo", "-u", "postgres", "psql", "-t", "-c",
			"SELECT datname FROM pg_database WHERE datistemplate = false AND datname NOT IN ('postgres');")
	} else {
		output, _ = runCmd("mysql", "-N", "-e",
			"SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('information_schema','mysql','performance_schema','sys');")
	}

	if strings.TrimSpace(output) == "" {
		fmt.Println("  │  No user databases found                    │")
	} else {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				fmt.Printf("  │  📦 %s\n", line)
			}
		}
	}
	fmt.Println("  ╰─────────────────────────────────────────────╯")
}
