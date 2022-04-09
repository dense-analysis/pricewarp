package model

import (
	"github.com/dense-analysis/pricewarp/internal/database"
)

// LoadList loads rows from a database into a list.
//
// The `scan` function determine how to set the values on each object.
func LoadList[T any](
	conn database.Queryable,
	list *[]T,
	capacity int,
	scan func(database.Row, *T) error,
	sql string,
	arguments ...any,
) error {
	rows, err := conn.Query(sql, arguments...)

	if err != nil {
		return err
	}

	*list = make([]T, 0, capacity)
	var instance T

	for rows.Next() {
		if err := scan(rows, &instance); err != nil {
			return err
		}

		*list = append(*list, instance)
	}

	return nil
}
