package database

import (
	"context"
	"os"
	"fmt"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgtype"
	shopspring "github.com/jackc/pgtype/ext/shopspring-numeric"
)

// Connect connects to the Postgres database with the project environment variables
func Connect() (*pgx.Conn, error) {
	conn, err := pgx.Connect(
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

	if err != nil {
		return nil, err
	}

	conn.ConnInfo().RegisterDataType(pgtype.DataType{
		Value: &shopspring.Numeric{},
		Name: "numeric",
		OID: pgtype.NumericOID,
	})

	return conn, err
}
