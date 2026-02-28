// Run read-only ClickHouse queries for database inspection.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/env"
)

const (
	truncatedStringLength = 70
	truncatedSliceLength  = 4
)

var writeKeywordPattern = regexp.MustCompile(
	`\b(insert|update|delete|drop|alter|truncate|create|optimize|system|attach|detach|rename|grant|revoke|kill)\b`,
)

var intoOutfilePattern = regexp.MustCompile(`\binto\s+outfile\b`)

func main() {
	query := ""
	full := false

	flag.StringVar(&query, "query", "", "A single SELECT/SHOW/EXPLAIN query to run")
	flag.StringVar(&query, "q", "", "A single SELECT/SHOW/EXPLAIN query to run")
	flag.BoolVar(&full, "full", false, "Show full output (values are truncated by default)")
	flag.Parse()

	normalizedQuery, err := validateReadOnlyQuery(query)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	env.LoadEnvironmentVariables()

	conn, err := database.Connect()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Connection error: %s\n", err)
		os.Exit(1)
	}

	defer func() {
		_ = conn.Close()
	}()

	columns, rows, err := runReadOnlyQuery(conn, normalizedQuery)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Query error: %s\n", err)
		os.Exit(1)
	}

	printRows(columns, rows, full)
}

func validateReadOnlyQuery(query string) (string, error) {
	normalized := strings.TrimSpace(query)

	if normalized == "" {
		return "", errors.New("query is required (use --query)")
	}

	if strings.HasSuffix(normalized, ";") {
		normalized = strings.TrimSpace(normalized[:len(normalized)-1])
	}

	lowered := strings.ToLower(normalized)

	if !strings.HasPrefix(lowered, "select") &&
		!strings.HasPrefix(lowered, "show") &&
		!strings.HasPrefix(lowered, "explain") {
		return "", errors.New("only SELECT, SHOW, and EXPLAIN queries are allowed")
	}

	if strings.Contains(normalized, ";") {
		return "", errors.New("only a single statement is allowed")
	}

	if intoOutfilePattern.MatchString(lowered) {
		return "", errors.New("INTO OUTFILE is not allowed")
	}

	if strings.HasPrefix(lowered, "select") || strings.HasPrefix(lowered, "explain") {
		if keyword := writeKeywordPattern.FindString(lowered); keyword != "" {
			return "", fmt.Errorf("keyword %q is not allowed", keyword)
		}
	}

	return normalized, nil
}

func runReadOnlyQuery(conn *database.Conn, query string) ([]string, [][]any, error) {
	rows, err := conn.Query(query)

	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	type rowsWithColumns interface {
		Columns() []string
		ColumnTypes() []chdriver.ColumnType
	}

	columnRows, ok := rows.(rowsWithColumns)

	if !ok {
		return nil, nil, errors.New("query rows do not expose column metadata")
	}

	columns := columnRows.Columns()
	columnTypes := columnRows.ColumnTypes()

	if len(columns) != len(columnTypes) {
		return nil, nil, errors.New("query column metadata is inconsistent")
	}

	results := make([][]any, 0)

	for rows.Next() {
		scanTargets := buildScanTargets(columnTypes)

		if err := rows.Scan(scanTargets...); err != nil {
			return nil, nil, err
		}

		values := extractScannedValues(scanTargets)
		results = append(results, normalizeScannedValues(values))
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return columns, results, nil
}

func buildScanTargets(columnTypes []chdriver.ColumnType) []any {
	scanTargets := make([]any, len(columnTypes))

	for index, columnType := range columnTypes {
		scanType := columnType.ScanType()

		if scanType == nil {
			var fallback string
			scanTargets[index] = &fallback
			continue
		}

		scanTargets[index] = reflect.New(scanType).Interface()
	}

	return scanTargets
}

func extractScannedValues(scanTargets []any) []any {
	values := make([]any, len(scanTargets))

	for index, target := range scanTargets {
		values[index] = unwrapPointerValue(target)
	}

	return values
}

func unwrapPointerValue(value any) any {
	reflectionValue := reflect.ValueOf(value)

	if !reflectionValue.IsValid() {
		return nil
	}

	for reflectionValue.Kind() == reflect.Pointer {
		if reflectionValue.IsNil() {
			return nil
		}

		reflectionValue = reflectionValue.Elem()
	}

	if !reflectionValue.IsValid() {
		return nil
	}

	return reflectionValue.Interface()
}

func normalizeScannedValues(values []any) []any {
	normalized := make([]any, len(values))

	for index, value := range values {
		switch typed := value.(type) {
		case []byte:
			copied := make([]byte, len(typed))
			copy(copied, typed)
			normalized[index] = string(copied)
		default:
			normalized[index] = typed
		}
	}

	return normalized
}

func printRows(columns []string, rows [][]any, full bool) {
	fmt.Printf("**%d rows returned**\n", len(rows))

	for rowIndex, row := range rows {
		fmt.Printf("\n## Row %d\n", rowIndex+1)

		for columnIndex, column := range columns {
			fmt.Printf("%s: %s\n", column, formatValue(row[columnIndex], full))
		}
	}
}

func formatValue(value any, full bool) string {
	if value == nil {
		return "null"
	}

	if timeValue, ok := value.(time.Time); ok {
		return timeValue.Format(time.RFC3339Nano)
	}

	if full {
		if stringer, ok := value.(fmt.Stringer); ok {
			return stringer.String()
		}

		return fmt.Sprintf("%v", value)
	}

	switch typed := value.(type) {
	case string:
		if len(typed) > truncatedStringLength {
			return strconv.Quote(typed[:truncatedStringLength-3] + "...")
		}

		return strconv.Quote(typed)
	case []byte:
		stringValue := string(typed)

		if len(stringValue) > truncatedStringLength {
			return strconv.Quote(stringValue[:truncatedStringLength-3] + "...")
		}

		return strconv.Quote(stringValue)
	}

	if stringer, ok := value.(fmt.Stringer); ok {
		return stringer.String()
	}

	reflectionValue := reflect.ValueOf(value)

	if reflectionValue.IsValid() && reflectionValue.Kind() == reflect.Slice &&
		reflectionValue.Type().Elem().Kind() != reflect.Uint8 &&
		reflectionValue.Len() > truncatedSliceLength {
		parts := make([]string, 0, truncatedSliceLength)

		for index := 0; index < truncatedSliceLength; index += 1 {
			parts = append(parts, formatValue(reflectionValue.Index(index).Interface(), false))
		}

		return "[" + strings.Join(parts, ", ") + ", ...]"
	}

	return fmt.Sprintf("%v", value)
}
