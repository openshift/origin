package storage

import (
	"reflect"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// Test_tokenLimiter_take makes sure that the -1/0/+1 boundary cases work with burst
// The exact values in "want" are generally irrelevant because they are controlled by rate.Limiter
// They must be kept under one minute to make sure we can detect a reservation failure
func Test_tokenLimiter_take(t *testing.T) {
	nowFunc := func() time.Time {
		return time.Time{} // current time is always 0
	}

	type args struct {
		n int
	}
	tests := []struct {
		name         string
		burst, limit int
		args         args
		want         time.Duration
	}{
		{
			name:  "take 0",
			burst: 1,
			limit: 1,
			args: args{
				n: 0,
			},
			want: 0,
		},
		{
			name:  "take -1",
			burst: 1,
			limit: 1,
			args: args{
				n: -1,
			},
			want: 0,
		},
		{
			name:  "take -1 based on burst",
			burst: 4,
			limit: 1,
			args: args{
				n: 3,
			},
			want: 3 * time.Second,
		},
		{
			name:  "take exact based on burst",
			burst: 4,
			limit: 1,
			args: args{
				n: 4,
			},
			want: 4 * time.Second,
		},
		{
			name:  "take +1 based on burst",
			burst: 4,
			limit: 1,
			args: args{
				n: 5,
			},
			want: 9 * time.Second,
		},
		{
			name:  "take +many based on burst",
			burst: 6,
			limit: 100,
			args: args{
				n: 21,
			},
			want: 570 * time.Millisecond,
		},
	}
	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			limiter := &tokenLimiter{burst: tt.burst, rateLimiter: rate.NewLimiter(rate.Limit(tt.limit), tt.burst), nowFunc: nowFunc}
			if got := limiter.take(tt.args.n); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tokenLimiter.take() = %v, want %v", got, tt.want)
			}
		})
	}
}
