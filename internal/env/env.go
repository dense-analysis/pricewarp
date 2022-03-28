package env

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// LoadEnvironmentVariables loads the .env file or crashes the program with an error
func LoadEnvironmentVariables() {
	if err := godotenv.Load(".env"); err != nil {
		fmt.Fprintf(os.Stderr, ".env error: %s\n", err)
		os.Exit(1)
	}
}
