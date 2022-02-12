// Create a user for logging in to the price alerting system
package main

import (
	"fmt"
	"os"
	"golang.org/x/crypto/bcrypt"
	"github.com/w0rp/pricewarp/internal/env"
	"github.com/w0rp/pricewarp/internal/database"
)

func main() {
	env.LoadEnvironmentVariables()

	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: adduser <username> <password>\n")
		os.Exit(1)
	}

	// TODO: Add email validation for username
	username := os.Args[1]
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
