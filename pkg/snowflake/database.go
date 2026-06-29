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
	"fmt"
	"strings"
)

type DatabaseInfo struct {
	Name    string `json:"name" db:"name"`
	Owner   string `json:"owner,omitempty" db:"owner"`
	Comment string `json:"comment,omitempty" db:"comment"`
}

func ShowDatabases(ctx context.Context, oauthToken string) ([]DatabaseInfo, error) {
	db, err := openConnection(oauthToken)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(ctx, "SHOW DATABASES;")
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
