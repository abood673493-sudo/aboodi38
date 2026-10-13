// Command bootstrap-admin creates the first (or an additional) panel
// administrator. It is the only supported way to get an admin row into the
// database - there is no seed data and no default credentials anywhere in
// this project.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"

	"ssh-manager-panel/internal/auth"
	"ssh-manager-panel/internal/db"
)

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection string")
	username := flag.String("username", "", "admin username (will prompt if omitted)")
	role := flag.String("role", "admin", "role: admin, operator, or viewer")
	flag.Parse()

	if *databaseURL == "" {
		fmt.Fprintln(os.Stderr, "error: --database-url or $DATABASE_URL is required")
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	if *username == "" {
		fmt.Print("Admin username: ")
		line, _ := reader.ReadString('\n')
		*username = strings.TrimSpace(line)
	}
	if *username == "" {
		fmt.Fprintln(os.Stderr, "error: username must not be empty")
		os.Exit(1)
	}

	fmt.Print("Password: ")
	pw1, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading password:", err)
		os.Exit(1)
	}
	fmt.Print("Confirm password: ")
	pw2, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading password:", err)
		os.Exit(1)
	}
	if string(pw1) != string(pw2) {
		fmt.Fprintln(os.Stderr, "error: passwords do not match")
		os.Exit(1)
	}
	password := string(pw1)

	if err := auth.ValidatePasswordStrength(password); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error hashing password:", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, *databaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error connecting to database:", err)
		os.Exit(1)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `
		INSERT INTO admins (username, password_hash, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (username) DO UPDATE SET password_hash = $2, role = $3, is_active = true, updated_at = now()`,
		*username, hash, *role)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating admin:", err)
		os.Exit(1)
	}

	fmt.Printf("Admin %q (%s) created/updated successfully.\n", *username, *role)
}
