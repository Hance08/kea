// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package utils

func AbsInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
