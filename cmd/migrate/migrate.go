// Package migrate runs migrations for the project
package main

import (
    "os"
    "fmt"
    "flag"
    "strconv"
    "strings"
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

// migrationNumber sets the migration to move to, the maximum number by default.
var migrationNumber = ^uint(0)

func readCliOptions() {
    flag.Parse()
    args := flag.Args()

    if len(args) > 0 {
        if len(args) > 1 {
            fmt.Fprintln(os.Stderr, "Too many arguments")
            os.Exit(1)
        }

        value, err := strconv.Atoi(args[0])

        if err != nil || value < 0 {
            fmt.Fprintln(os.Stderr, "Invalid migration number")
            os.Exit(1)
        }

        migrationNumber = uint(value)
    }
}

func main() {
    readCliOptions()

    host := os.Getenv("PGHOST")
    username := os.Getenv("PGUSER")
    database := os.Getenv("PGDATABASE")
    password := os.Getenv("PGPASSWORD")

    fileURL := "file://migrations"
    databaseURL := fmt.Sprintf(
        "postgres://%s:%s@%s/%s",
        username,
        password,
        host,
        database,
    )

    instance, newErr := migrate.New(fileURL, databaseURL)

    if newErr != nil {
        fmt.Fprintln(os.Stderr, newErr.Error())
        os.Exit(1)
    }

    currentVersion, _, _ := instance.Version()
    var migrationError error

    for currentVersion != migrationNumber {
        nextVersion := currentVersion + 1

        if migrationNumber < currentVersion {
            nextVersion = currentVersion - 1
        }

        var err error

        if (nextVersion == 0) {
            err = instance.Down()
        } else {
            err = instance.Migrate(nextVersion)
        }

        // Migrate one step at a time so we can save the last migration number
        // which we ran without errors.
        if err != nil {
            if !strings.Contains(err.Error(), "no migration found") {
                migrationError = err
            }

            // Force the version on errors, so we can just fix the problems in
            // the SQL files, which always use transactions, and run them again.
            if (currentVersion == 0) {
                instance.Force(-1)
            } else {
                instance.Force(int(currentVersion))
            }

            break
        }

        currentVersion = nextVersion
    }

    if migrationError != nil {
        fmt.Fprintln(os.Stderr, migrationError.Error())
        os.Exit(1)
    }
}
