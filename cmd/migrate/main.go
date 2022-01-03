// Migrate the database from one state to another
package main

import (
	"math"
	"os"
	"fmt"
	"strconv"
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"github.com/jackc/pgx/v4"
	"github.com/joho/godotenv"
)


type MigrationExecutor struct {
	connection *pgx.Conn
	directoryName string
	migrationFileList []string
}

func NewMigrationExecutor(connection *pgx.Conn, directoryName string) (*MigrationExecutor, error) {
	fileList, err := ioutil.ReadDir(directoryName)

	if err != nil {
		return nil, err
	}

	migrationFileList := make([]string, 0, len(fileList))

	for _, file := range fileList {
		if !file.IsDir() {
			migrationFileList = append(migrationFileList, file.Name())
		}
	}

	return &MigrationExecutor{connection, directoryName, migrationFileList}, nil
}

func (executor *MigrationExecutor) CreateMigrationTable() error {
	_, err := executor.connection.Exec(
		context.Background(),
		"CREATE TABLE IF NOT EXISTS crypto_migration (id serial, migration_number integer NOT NULL UNIQUE);",
	)

	return err
}

func (executor *MigrationExecutor) CurrentMigration() (int, error) {
	row := executor.connection.QueryRow(
		context.Background(),
		"SELECT COALESCE(MAX(migration_number), 0) FROM crypto_migration;",
	)

	var migrationNumber int32
	err := row.Scan(&migrationNumber)

	return int(migrationNumber), err
}

func (executor *MigrationExecutor) applyMigration(migrationNumber int, reverse bool) (bool, error) {
	var matchedFilename string

	for _, filename := range executor.migrationFileList {
		splitList := strings.Split(filename, "_")
		fileMigrationNumber, _ := strconv.Atoi(splitList[0])
		isReverseFile := splitList[len(splitList) - 1] == "reverse.sql"

		if migrationNumber == fileMigrationNumber && reverse == isReverseFile {
			matchedFilename = filepath.Join(executor.directoryName, filename)
			break
		}
	}

	if len(matchedFilename) == 0 {
		return true, nil
	}

	fmt.Printf("Applying migration: %s\n", matchedFilename)

	file, readErr := ioutil.ReadFile(matchedFilename)

	if readErr != nil {
		return false, readErr
	}

	batch := &pgx.Batch{}
	// NOTE: SQL functions in migration files won't work.
	queries := strings.Split(string(file), ";\n")

	for _, query := range queries {
		batch.Queue(query)
	}

	if reverse {
		batch.Queue(
			"DELETE FROM crypto_migration WHERE migration_number = $1;",
			&migrationNumber,
		)
	} else {
		batch.Queue(
			"INSERT INTO crypto_migration (migration_number) VALUES ($1) ON CONFLICT DO NOTHING;",
			&migrationNumber,
		)
	}

	results := executor.connection.SendBatch(context.Background(), batch)

	if _, err := results.Exec(); err != nil {
		return false, err
	}

	return false, nil
}

func (executor *MigrationExecutor) ApplyMigrations(selectedMigrationNumber int) error {
	if err := executor.CreateMigrationTable(); err != nil {
		return err
	}

	startMigrationNumber, currentErr := executor.CurrentMigration()

	if currentErr != nil {
		return currentErr
	}

	reverse := false

	if selectedMigrationNumber < startMigrationNumber {
		reverse = true
	}

	for i := startMigrationNumber; i != selectedMigrationNumber; {
		if !reverse {
			i += 1
		}

		stop, err := executor.applyMigration(i, reverse)

		if reverse {
			i -= 1
		}

		if err != nil {
			return err
		}

		if stop {
			break
		}
	}

	return nil
}

func parseSelectedMigration() int {
	selectedMigration := math.MaxInt64

	if len(os.Args) > 2 {
		fmt.Fprintf(os.Stderr, "Too many arguments\n")
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		var err error
		selectedMigration, err = strconv.Atoi(os.Args[1])

		if err != nil || selectedMigration < 0 {
			fmt.Fprintf(os.Stderr, "Invalid migration number: %s\n", os.Args[1])
			os.Exit(1)
		}
	}

	return selectedMigration
}

func main() {
	selectedMigration := parseSelectedMigration()

	if err := godotenv.Load(".env"); err != nil {
		fmt.Fprintf(os.Stderr, ".env error: %s\n", err)
		os.Exit(1)
	}

	conn, connectionErr := pgx.Connect(
		context.Background(),
		fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s",
			os.Getenv("DB_USERNAME"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT"),
			os.Getenv("DB_NAME"),
		),
	)

	if connectionErr != nil {
		fmt.Fprintf(os.Stderr, "Connection error: %s\n", connectionErr)
		os.Exit(1)
	}

	executor, executorErr := NewMigrationExecutor(conn, "migrations")

	if executorErr != nil {
		fmt.Fprintf(os.Stderr, "Error loading migrations: %s\n", executorErr)
		os.Exit(1)
	}

	if err := executor.ApplyMigrations(selectedMigration); err != nil {
		fmt.Fprintf(os.Stderr, "Error applying migration: %s\n", err)
		os.Exit(1)
	}
}
