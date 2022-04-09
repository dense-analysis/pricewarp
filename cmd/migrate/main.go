// Migrate the database from one state to another
package main

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/env"
)

type MigrationExecutor struct {
	connection        *database.Conn
	directoryName     string
	migrationFileList []string
}

func NewMigrationExecutor(connection *database.Conn, directoryName string) (*MigrationExecutor, error) {
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
	return executor.connection.Exec(
		"CREATE TABLE IF NOT EXISTS crypto_migration (id serial, migration_number integer NOT NULL UNIQUE);",
	)
}

func (executor *MigrationExecutor) CurrentMigration() (int, error) {
	row := executor.connection.QueryRow(
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
		isReverseFile := splitList[len(splitList)-1] == "reverse.sql"

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

	batch := &database.Batch{}
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

	results := executor.connection.SendBatch(batch)

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

	env.LoadEnvironmentVariables()

	conn, connectionErr := database.Connect()

	if connectionErr != nil {
		fmt.Fprintf(os.Stderr, "Connection error: %s\n", connectionErr)
		os.Exit(1)
	}

	defer conn.Close()

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
