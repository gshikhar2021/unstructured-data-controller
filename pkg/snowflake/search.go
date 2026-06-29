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
)

type ChunkResult struct {
	ChunkIndex int     `json:"chunk_index"`
	ChunkText  string  `json:"chunk_text"`
	Score      float64 `json:"score"`
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
