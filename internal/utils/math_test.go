// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package utils_test

import (
	"math"
	"testing"

	"github.com/hance08/kea/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestAbsInt64(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want int64
	}{
		{"positive", 42, 42},
		{"negative", -42, 42},
		{"zero", 0, 0},
		{"max int64", math.MaxInt64, math.MaxInt64},
		{"negative one", -1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, utils.AbsInt64(tt.n))
		})
	}
}

func TestAbsInt64_MinInt64_Panics(t *testing.T) {
	assert.Panics(t, func() {
		utils.AbsInt64(math.MinInt64)
	})
}
