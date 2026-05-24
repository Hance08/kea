// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

const sqliteChunkSize = 500

func chunkInt64(ids []int64, size int) [][]int64 {
	if size <= 0 {
		panic("chunkInt64: size must be positive")
	}
	if len(ids) == 0 {
		return nil
	}
	chunks := make([][]int64, 0, (len(ids)+size-1)/size)
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[i:end])
	}
	return chunks
}
