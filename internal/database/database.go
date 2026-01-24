// Package database wraps the database implementation used for Pricewarp.
package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"os"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type Conn struct {
	chConn clickhouse.Conn
}

type Row interface {
	Scan(dest ...any) error
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
	Err() error
}

type Batch interface {
	Append(values ...any) error
	Send() error
}

var ErrNoRows = sql.ErrNoRows

// Connect connects to the ClickHouse database with the project environment variables.
func Connect() (*Conn, error) {
	address := fmt.Sprintf("%s:%s", os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{address},
		Auth: clickhouse.Auth{
			Database: os.Getenv("DB_NAME"),
			Username: os.Getenv("DB_USERNAME"),
			Password: os.Getenv("DB_PASSWORD"),
		},
		DialTimeout: time.Second * 5,
	})

	if err != nil {
		return nil, err
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, err
	}

	return &Conn{chConn: conn}, nil
}

// Close closes a database connection.
func (conn *Conn) Close() error {
	return conn.chConn.Close()
}

// Exec executes a database query.
func (conn *Conn) Exec(sql string, arguments ...any) error {
	return conn.chConn.Exec(context.Background(), sql, arguments...)
}

// Query executes a database query.
func (conn *Conn) Query(sql string, arguments ...any) (Rows, error) {
	return conn.chConn.Query(context.Background(), sql, arguments...)
}

// QueryRow executes a database query returning Row data.
func (conn *Conn) QueryRow(sql string, arguments ...any) Row {
	return conn.chConn.QueryRow(context.Background(), sql, arguments...)
}

// PrepareBatch prepares an insert batch for ClickHouse.
func (conn *Conn) PrepareBatch(sql string) (Batch, error) {
	return conn.chConn.PrepareBatch(context.Background(), sql)
}

// Queryable defines an interface for a connection.
type Queryable interface {
	Exec(sql string, arguments ...any) error
	Query(sql string, arguments ...any) (Rows, error)
	QueryRow(sql string, arguments ...any) Row
}

// HashID returns a stable int64 identifier for the provided value.
func HashID(value string) int64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(value))

	return int64(hasher.Sum64())
}

// RandomID generates a random int64 identifier.
func RandomID() (int64, error) {
	var buffer [8]byte
	_, err := rand.Read(buffer[:])

	if err != nil {
		return 0, err
	}

	return int64(binary.BigEndian.Uint64(buffer[:])), nil
}
