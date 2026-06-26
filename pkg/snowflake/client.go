/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package snowflake

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/snowflakedb/gosnowflake"
)

// DatabaseInfo contains information about a Snowflake database
type DatabaseInfo struct {
	Name    string `json:"name" db:"name"`
	Owner   string `json:"owner,omitempty" db:"owner"`
	Comment string `json:"comment,omitempty" db:"comment"`
}

// ShowDatabases connects to Snowflake using the provided OAuth token,
func ShowDatabases(ctx context.Context, oauthToken string) ([]DatabaseInfo, error) {
	// Get Snowflake connection parameters from environment
	account := os.Getenv("SNOWFLAKE_ACCOUNT")
	if account == "" {
		return nil, errors.New("SNOWFLAKE_ACCOUNT environment variable not set")
	}

	cfg := &gosnowflake.Config{
		Account:       account,
		Authenticator: gosnowflake.AuthTypeOAuth,
		Token:         oauthToken,
		OCSPFailOpen:  gosnowflake.OCSPFailOpenFalse,
	}

	dsn, err := gosnowflake.DSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build snowflake DSN: %w", err)
	}

	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create snowflake connection: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Configure connection pool for security
	// MaxOpenConns=1: Single connection per request (we open/close immediately)
	// MaxIdleConns=0: Don't keep connections idle (reduces token exposure time)
	// ConnMaxLifetime=5min: Close connections after 5 minutes (limits token lifetime)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to snowflake: %w", err)
	}

	// Run SHOW DATABASES query
	// This will return only databases the authenticated user has access to (RBAC enforced by Snowflake)
	rows, err := db.QueryContext(ctx, "SHOW DATABASES LIKE 'SHIKHAR%';")
	if err != nil {
		return nil, fmt.Errorf("failed to execute SHOW DATABASES: %w", err)
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %w", err)
	}

	var databases []DatabaseInfo

	for rows.Next() {
		values := make([]any, len(columns))
		for i := range values {
			var placeholder any
			values[i] = &placeholder
		}

		if err := rows.Scan(values...); err != nil {
			return nil, fmt.Errorf("failed to scan database row: %w", err)
		}

		var db DatabaseInfo
		for i, colName := range columns {
			val := *(values[i].(*any))
			if val == nil {
				continue
			}
			s := fmt.Sprintf("%s", val)
			switch strings.ToLower(colName) {
			case "name":
				db.Name = s
			case "owner":
				db.Owner = s
			case "comment":
				db.Comment = s
			default:
			}
		}
		databases = append(databases, db)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating database rows: %w", err)
	}

	return databases, nil
}
