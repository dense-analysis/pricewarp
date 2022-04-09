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

	defer conn.Close()

	err = conn.Exec(
		"insert into crypto_user(username, password) values($1, $2)",
		username,
		string(passwordHash),
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Query error: %s\n", err)
		os.Exit(1)
	}
}
