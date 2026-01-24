// Create a user for logging in to the price alerting system
package main

import (
	"fmt"
	"net/mail"
	"os"

	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/env"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	env.LoadEnvironmentVariables()

	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: adduser <username> <password>\n")
		os.Exit(1)
	}

	username := os.Args[1]
	_, err := mail.ParseAddress(username)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Username is not a valid email address.\n")
		os.Exit(1)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(os.Args[2]), 14)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Password hashing error: %s\n", err)
		os.Exit(1)
	}

	conn, err := database.Connect()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Connection error: %s\n", err)
		os.Exit(1)
	}

	defer func() {
		_ = conn.Close()
	}()

	userID := database.HashID(username)
	var existingID int64

	row := conn.QueryRow(
		`select user_id
		from crypto_users
		where username = ? and is_active = 1
		order by updated_at desc
		limit 1`,
		username,
	)

	if err := row.Scan(&existingID); err == nil {
		fmt.Fprintf(os.Stderr, "User already exists.\n")
		os.Exit(1)
	} else if err != database.ErrNoRows {
		fmt.Fprintf(os.Stderr, "Query error: %s\n", err)
		os.Exit(1)
	}

	err = conn.Exec(
		`insert into crypto_users
			(user_id, username, password_hash, created_at, updated_at, is_active)
		values (?, ?, ?, now64(9), now64(9), 1)`,
		userID,
		username,
		string(passwordHash),
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Query error: %s\n", err)
		os.Exit(1)
	}
}
