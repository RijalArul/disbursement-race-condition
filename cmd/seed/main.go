package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/RijalArul/disbursement-race-condition/internal/config"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
)

// seedUsers matches the demo credentials fixed by the take-home spec.
// These are intentionally committed — see README for the production caveat.
var seedUsers = []struct {
	Username string
	Password string
	Role     domain.UserRole
}{
	{"superadmin", "superadmin123", domain.RoleSuperAdmin},
	{"admin", "admin123", domain.RoleAdmin},
	{"operator", "operator123", domain.RoleOperator},
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect error:", err)
		os.Exit(1)
	}

	for _, u := range seedUsers {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), 10)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hash error:", err)
			os.Exit(1)
		}

		result := db.Exec(
			`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)
			 ON CONFLICT (username) DO NOTHING`,
			u.Username, string(hash), u.Role,
		)
		if result.Error != nil {
			fmt.Fprintln(os.Stderr, "seed error for", u.Username, ":", result.Error)
			os.Exit(1)
		}
		if result.RowsAffected > 0 {
			fmt.Println("seeded:", u.Username)
		} else {
			fmt.Println("already exists, skipped:", u.Username)
		}
	}
}
