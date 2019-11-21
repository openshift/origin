// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package primitive

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTimestampCompare(t *testing.T) {
	testcases := []struct {
		name     string
		tp       Timestamp
		tp2      Timestamp
		expected int
	}{
		{"equal", Timestamp{T: 12345, I: 67890}, Timestamp{T: 12345, I: 67890}, 0},
		{"T greater than", Timestamp{T: 12345, I: 67890}, Timestamp{T: 2345, I: 67890}, 1},
		{"I greater than", Timestamp{T: 12345, I: 67890}, Timestamp{T: 12345, I: 7890}, 1},
		{"T less than", Timestamp{T: 12345, I: 67890}, Timestamp{T: 112345, I: 67890}, -1},
		{"I less than", Timestamp{T: 12345, I: 67890}, Timestamp{T: 12345, I: 167890}, -1},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			result := CompareTimestamp(tc.tp, tc.tp2)
			require.Equal(t, tc.expected, result)
		})
	}
}
