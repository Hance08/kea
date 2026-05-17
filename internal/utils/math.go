// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package utils

import "math"

func AbsInt64(n int64) int64 {
	if n == math.MinInt64 {
		panic("utils.AbsInt64: undefined for math.MinInt64 (overflow)")
	}
	if n < 0 {
		return -n
	}
	return n
}
