package main

import (
	"reflect"
	"strings"
	"testing"

	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type fakeColumnType struct {
	name     string
	scanType reflect.Type
}

func (column fakeColumnType) Name() string {
	return column.name
}

func (column fakeColumnType) Nullable() bool {
	return false
}

func (column fakeColumnType) ScanType() reflect.Type {
	return column.scanType
}

func (column fakeColumnType) DatabaseTypeName() string {
	return ""
}

func TestValidateReadOnlyQuery(t *testing.T) {
	testCases := []struct {
		Name      string
		Query     string
		Want      string
		WantError string
	}{
		{
			Name:  "select query with trailing semicolon",
			Query: " SELECT 1; ",
			Want:  "SELECT 1",
		},
		{
			Name:  "show query allowed",
			Query: "SHOW CREATE TABLE crypto_currency_prices",
			Want:  "SHOW CREATE TABLE crypto_currency_prices",
		},
		{
			Name:  "explain query allowed",
			Query: "EXPLAIN SELECT * FROM crypto_currencies",
			Want:  "EXPLAIN SELECT * FROM crypto_currencies",
		},
		{
			Name:      "missing query",
			Query:     "",
			WantError: "query is required (use --query)",
		},
		{
			Name:      "write query start rejected",
			Query:     "INSERT INTO crypto_currencies VALUES ('BTC','BTC',now())",
			WantError: "only SELECT, SHOW, and EXPLAIN queries are allowed",
		},
		{
			Name:      "multiple statements rejected",
			Query:     "SELECT 1; SELECT 2",
			WantError: "only a single statement is allowed",
		},
		{
			Name:      "into outfile rejected",
			Query:     "SELECT * FROM crypto_currencies INTO OUTFILE '/tmp/out'",
			WantError: "INTO OUTFILE is not allowed",
		},
		{
			Name:      "write keyword in select rejected",
			Query:     "SELECT * FROM crypto_currencies WHERE ticker = 'drop'",
			WantError: "keyword \"drop\" is not allowed",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			got, err := validateReadOnlyQuery(testCase.Query)

			if testCase.WantError != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", testCase.WantError)
				}

				if err.Error() != testCase.WantError {
					t.Fatalf("expected error %q, got %q", testCase.WantError, err.Error())
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != testCase.Want {
				t.Fatalf("expected query %q, got %q", testCase.Want, got)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	longText := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"

	truncated := formatValue(longText, false)

	if !strings.HasPrefix(truncated, "\"") || !strings.HasSuffix(truncated, "...\"") {
		t.Fatalf("unexpected truncated output: %s", truncated)
	}

	if got := formatValue([]int{1, 2, 3, 4, 5, 6}, false); got != "[1, 2, 3, 4, ...]" {
		t.Fatalf("unexpected slice output: %s", got)
	}
}

func TestBuildAndExtractScanTargets(t *testing.T) {
	columnTypes := []chdriver.ColumnType{
		fakeColumnType{name: "ticker", scanType: reflect.TypeOf("")},
		fakeColumnType{name: "count", scanType: reflect.TypeOf(int64(0))},
		fakeColumnType{name: "tags", scanType: reflect.TypeOf([]string{})},
		fakeColumnType{name: "nullable", scanType: reflect.TypeOf((*string)(nil))},
	}
	targets := buildScanTargets(columnTypes)

	*targets[0].(*string) = "BTC"
	*targets[1].(*int64) = 3
	*targets[2].(*[]string) = []string{"A", "B"}

	nullableValue := "value"
	*targets[3].(**string) = &nullableValue

	values := extractScannedValues(targets)

	if values[0] != "BTC" {
		t.Fatalf("unexpected first value: %#v", values[0])
	}

	if values[1] != int64(3) {
		t.Fatalf("unexpected second value: %#v", values[1])
	}

	wantSlice := []string{"A", "B"}
	if !reflect.DeepEqual(values[2], wantSlice) {
		t.Fatalf("unexpected third value: %#v", values[2])
	}

	if values[3] != "value" {
		t.Fatalf("unexpected nullable value: %#v", values[3])
	}
}
