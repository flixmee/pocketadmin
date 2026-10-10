//go:build !no_default_driver

package core

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"

	"github.com/pocketbase/dbx"
	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction(
		"pb_vec_cosine_distance",
		2,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("pb_vec_cosine_distance expects 2 arguments")
			}

			left, err := decodeSQLiteVector(args[0])
			if err != nil {
				return nil, err
			}
			right, err := decodeSQLiteVector(args[1])
			if err != nil {
				return nil, err
			}
			if len(left) == 0 || len(left) != len(right) {
				return nil, nil
			}

			var dot, leftNorm, rightNorm float64
			for i, v := range left {
				r := right[i]
				dot += v * r
				leftNorm += v * v
				rightNorm += r * r
			}
			if leftNorm == 0 || rightNorm == 0 {
				return nil, nil
			}

			return 1 - dot/(math.Sqrt(leftNorm)*math.Sqrt(rightNorm)), nil
		},
	)
}

func decodeSQLiteVector(value driver.Value) ([]float64, error) {
	var raw []byte
	switch v := value.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported vector value %T", value)
	}

	var result []float64
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("invalid vector JSON: %w", err)
	}

	return result, nil
}

func DefaultDBConnect(dbPath string) (*dbx.DB, error) {
	// Note: the busy_timeout pragma must be first because
	// the connection needs to be set to block on busy before WAL mode
	// is set in case it hasn't been already set by another connection.
	pragmas := "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=journal_size_limit(200000000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-32000)&_defensive=1"

	db, err := dbx.Open("sqlite", dbPath+pragmas)
	if err != nil {
		return nil, err
	}

	return db, nil
}
