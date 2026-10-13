package authentication

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

// recordWrapper returns a Wrapper that appends "<name>:before" before calling next and
// "<name>:after" after, so tests can assert composition order. failBefore/failAfter make it
// return an error at the corresponding point.
func recordWrapper(log *[]string, name string, failBefore, failAfter bool) Wrapper {
	return func(ctx context.Context, next func() error) error {
		*log = append(*log, name+":before")
		if failBefore {
			return fmt.Errorf("%s before error", name)
		}
		if err := next(); err != nil {
			return err
		}
		*log = append(*log, name+":after")
		if failAfter {
			return fmt.Errorf("%s after error", name)
		}
		return nil
	}
}

func TestChain(t *testing.T) {
	tests := []struct {
		name      string
		wrappers  func(log *[]string) []Wrapper
		finalErr  bool
		wantOrder []string
		wantErr   bool
	}{
		{
			name:      "empty chain runs only final",
			wrappers:  func(*[]string) []Wrapper { return nil },
			wantOrder: []string{"final"},
		},
		{
			name: "single wrapper brackets final",
			wrappers: func(log *[]string) []Wrapper {
				return []Wrapper{recordWrapper(log, "A", false, false)}
			},
			wantOrder: []string{"A:before", "final", "A:after"},
		},
		{
			name: "first wrapper is outermost",
			wrappers: func(log *[]string) []Wrapper {
				return []Wrapper{
					recordWrapper(log, "A", false, false),
					recordWrapper(log, "B", false, false),
				}
			},
			wantOrder: []string{"A:before", "B:before", "final", "B:after", "A:after"},
		},
		{
			name: "outer wrapper failing before next short-circuits inner and final",
			wrappers: func(log *[]string) []Wrapper {
				return []Wrapper{
					recordWrapper(log, "A", true, false),
					recordWrapper(log, "B", false, false),
				}
			},
			wantOrder: []string{"A:before"},
			wantErr:   true,
		},
		{
			name: "inner wrapper failing before next runs no final and no afters",
			wrappers: func(log *[]string) []Wrapper {
				return []Wrapper{
					recordWrapper(log, "A", false, false),
					recordWrapper(log, "B", true, false),
				}
			},
			wantOrder: []string{"A:before", "B:before"},
			wantErr:   true,
		},
		{
			name: "final error propagates and skips afters",
			wrappers: func(log *[]string) []Wrapper {
				return []Wrapper{
					recordWrapper(log, "A", false, false),
					recordWrapper(log, "B", false, false),
				}
			},
			finalErr:  true,
			wantOrder: []string{"A:before", "B:before", "final"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log []string
			final := func() error {
				log = append(log, "final")
				if tt.finalErr {
					return fmt.Errorf("final error")
				}
				return nil
			}

			err := Chain(tt.wrappers(&log)...)(context.Background(), final)

			if tt.wantErr != (err != nil) {
				t.Fatalf("Chain error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(log, tt.wantOrder) {
				t.Fatalf("execution order = %v, want %v", log, tt.wantOrder)
			}
		})
	}
}
