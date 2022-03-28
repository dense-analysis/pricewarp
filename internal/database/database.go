// Package database wraps the database implmementation used for Pricewarp
package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgtype"
	shopspring "github.com/jackc/pgtype/ext/shopspring-numeric"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Conn struct {
	pgxConn *pgxpool.Conn
}
type Tx struct {
	pgxTx pgx.Tx
}

type Row = pgx.Row
type Rows = pgx.Rows
type Batch = pgx.Batch
type BatchResults = pgx.BatchResults

var ErrNoRows = pgx.ErrNoRows

func afterConnect(context context.Context, conn *pgx.Conn) error {
	// Set up a decimal type for prices.
	conn.ConnInfo().RegisterDataType(pgtype.DataType{
		Value: &shopspring.Numeric{},
		Name:  "numeric",
		OID:   pgtype.NumericOID,
	})

	return nil
}

// Connect connects to the Postgres database with the project environment variables
func Connect() (*Conn, error) {
	config, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		os.Getenv("DB_USERNAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	))

	if err != nil {
		return nil, err
	}

	config.AfterConnect = afterConnect

	pool, err := pgxpool.ConnectConfig(context.Background(), config)

	if err != nil {
		return nil, err
	}

	conn, err := pool.Acquire(context.Background())

	if err != nil {
		return nil, err
	}

	return &Conn{conn}, nil
}

// Close closes a database connection
func (conn *Conn) Close() {
	conn.pgxConn.Release()
}

// Exec executes a database query
func (conn *Conn) Exec(sql string, arguments ...any) error {
	_, err := conn.pgxConn.Exec(context.Background(), sql, arguments...)

	return err
}

// Query executes a database query
func (conn *Conn) Query(sql string, arguments ...any) (Rows, error) {
	return conn.pgxConn.Query(context.Background(), sql, arguments...)
}

// QueryRow executes a database query returning Row data
func (conn *Conn) QueryRow(sql string, arguments ...any) Row {
	return conn.pgxConn.QueryRow(context.Background(), sql, arguments...)
}

// SendBatch send a series of queries in a batch.
func (conn *Conn) SendBatch(batch *Batch) BatchResults {
	return conn.pgxConn.SendBatch(context.Background(), batch)
}

// CopyFrom copies rows into a database.
func (conn *Conn) CopyFrom(tableName string, columNames []string, rows [][]any) (int64, error) {
	return conn.pgxConn.CopyFrom(context.Background(), pgx.Identifier{tableName}, columNames, pgx.CopyFromRows(rows))
}

// Begin starts a new transaction
func (conn *Conn) Begin() (*Tx, error) {
	tx, err := conn.pgxConn.Begin(context.Background())

	if err != nil {
		return nil, err
	}

	return &Tx{tx}, err
}

// Commit commits a transaction to the database
func (tx *Tx) Commit() error {
	return tx.pgxTx.Commit(context.Background())
}

// Rollback cancels a transaction in the database
func (tx *Tx) Rollback() error {
	return tx.pgxTx.Rollback(context.Background())
}

// Exec executes a database query
func (tx *Tx) Exec(sql string, arguments ...any) error {
	_, err := tx.pgxTx.Exec(context.Background(), sql, arguments...)

	return err
}

// Query executes a database query
func (tx *Tx) Query(sql string, arguments ...any) (Rows, error) {
	return tx.pgxTx.Query(context.Background(), sql, arguments...)
}

// QueryRow executes a database query returning Row data
func (tx *Tx) QueryRow(sql string, arguments ...any) Row {
	return tx.pgxTx.QueryRow(context.Background(), sql, arguments...)
}

// SendBatch send a series of queries in a batch.
func (tx *Tx) SendBatch(batch *Batch) BatchResults {
	return tx.pgxTx.SendBatch(context.Background(), batch)
}

// CopyFrom copies rows into a database.
func (tx *Tx) CopyFrom(tableName string, columNames []string, rows [][]interface{}) (int64, error) {
	return tx.pgxTx.CopyFrom(context.Background(), pgx.Identifier{tableName}, columNames, pgx.CopyFromRows(rows))
}

// Queryable defines an interface for either a connection or a transaction
type Queryable interface {
	// Exec executes a database query
	Exec(sql string, arguments ...any) error
	// QueryRow executes a database query
	Query(sql string, arguments ...any) (Rows, error)
	// QueryRow executes a database query returning Row data
	QueryRow(sql string, arguments ...any) Row
	// SendBatch send a series of queries in a batch.
	SendBatch(batch *Batch) BatchResults
	// CopyFrom copies rows into a database.
	CopyFrom(tableName string, columNames []string, rows [][]interface{}) (int64, error)
}
