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

// ChunkResult contains a chunk returned by vector similarity search
type ChunkResult struct {
	ChunkIndex int     `json:"chunk_index"`
	ChunkText  string  `json:"chunk_text"`
	Score      float64 `json:"score"`
}

// DatabaseInfo contains information about a Snowflake database
type DatabaseInfo struct {
	Name    string `json:"name" db:"name"`
	Owner   string `json:"owner,omitempty" db:"owner"`
	Comment string `json:"comment,omitempty" db:"comment"`
}

func openConnection(oauthToken string) (*sql.DB, error) {
	account := os.Getenv("SNOWFLAKE_ACCOUNT")
	if account == "" {
		return nil, errors.New("SNOWFLAKE_ACCOUNT environment variable not set")
	}

	cfg := &gosnowflake.Config{
		Account:       account,
		Authenticator: gosnowflake.AuthTypeOAuth,
		Token:         oauthToken,
		OCSPFailOpen:  gosnowflake.OCSPFailOpenTrue,
	}

	dsn, err := gosnowflake.DSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build snowflake DSN: %w", err)
	}

	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create snowflake connection: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

func ShowDatabases(ctx context.Context, oauthToken string) ([]DatabaseInfo, error) {
	db, err := openConnection(oauthToken)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

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

func SearchChunks(
	ctx context.Context, oauthToken, database, schema, table, vectorLiteral string,
) ([]ChunkResult, error) {
	db, err := openConnection(oauthToken)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	query := fmt.Sprintf(
		`SELECT CHUNK_INDEX, CHUNK_TEXT, VECTOR_COSINE_SIMILARITY(EMBEDDING, %s::VECTOR(FLOAT,768)) AS score `+
			`FROM %s.%s.%s ORDER BY score DESC LIMIT 5`,
		vectorLiteral, database, schema, table,
	)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute vector search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var chunks []ChunkResult
	for rows.Next() {
		var c ChunkResult
		if err := rows.Scan(&c.ChunkIndex, &c.ChunkText, &c.Score); err != nil {
			return nil, fmt.Errorf("failed to scan chunk row: %w", err)
		}
		chunks = append(chunks, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chunk rows: %w", err)
	}

	return chunks, nil
}
