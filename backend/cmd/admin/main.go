// admin — admin/moderator hisobini yaratish yoki parolini yangilash.
//
//	ADMIN_PASSWORD='...' go run ./cmd/admin -email admin@example.com -name "Ism" -role admin
//
// ADMIN_PASSWORD berilmasa parol stdin dan bir qator sifatida o'qiladi.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"yoldosh/internal/api"
	"yoldosh/internal/auth"
	"yoldosh/internal/platform"
	"yoldosh/internal/store"
)

func main() {
	email := flag.String("email", "", "email (majburiy)")
	name := flag.String("name", "", "ism")
	role := flag.String("role", "moderator", "moderator | admin")
	flag.Parse()
	if err := run(strings.TrimSpace(*email), *name, *role); err != nil {
		fmt.Fprintln(os.Stderr, "xato:", err)
		os.Exit(1)
	}
}

func run(email, name, role string) error {
	if email == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("-email kerak")
	}
	if !auth.IsAdminRole(role) {
		return fmt.Errorf("-role: moderator | admin")
	}
	pw := os.Getenv("ADMIN_PASSWORD")
	if pw == "" {
		fmt.Fprint(os.Stderr, "Parol: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		pw = strings.TrimRight(line, "\r\n")
	}
	if err := auth.CheckPasswordStrength(pw); err != nil {
		return err
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return err
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL o'rnatilmagan")
	}
	ctx := context.Background()
	db, err := platform.OpenDB(ctx, url)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := platform.Migrate(ctx, db); err != nil {
		return err
	}
	if err := store.New(db).UpsertAdmin(ctx, api.NewAdminID(), email, name, hash, role); err != nil {
		return err
	}
	fmt.Printf("tayyor: %s (%s)\n", email, role)
	return nil
}
