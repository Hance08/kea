// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkInt64_Empty(t *testing.T) {
	chunks := chunkInt64(nil, 500)
	assert.Empty(t, chunks)
}

func TestChunkInt64_UnderLimit(t *testing.T) {
	ids := []int64{1, 2, 3}
	chunks := chunkInt64(ids, 500)
	assert.Equal(t, [][]int64{{1, 2, 3}}, chunks)
}

func TestChunkInt64_ExactLimit(t *testing.T) {
	ids := make([]int64, 500)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	chunks := chunkInt64(ids, 500)
	assert.Len(t, chunks, 1)
	assert.Len(t, chunks[0], 500)
}

func TestChunkInt64_OverLimit(t *testing.T) {
	ids := make([]int64, 1300)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	chunks := chunkInt64(ids, 500)
	assert.Len(t, chunks, 3)
	assert.Len(t, chunks[0], 500)
	assert.Len(t, chunks[1], 500)
	assert.Len(t, chunks[2], 300)
	assert.Equal(t, int64(1), chunks[0][0])
	assert.Equal(t, int64(501), chunks[1][0])
	assert.Equal(t, int64(1001), chunks[2][0])
}
