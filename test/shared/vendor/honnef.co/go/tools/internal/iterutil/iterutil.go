package iterutil

import "iter"

func All[T any](seq iter.Seq[T], fn func(T) bool) bool {
	for v := range seq {
		if !fn(v) {
			return false
		}
	}
	return true
}

func Any[T any](seq iter.Seq[T], fn func(T) bool) bool {
	for v := range seq {
		if fn(v) {
			return true
		}
	}
	return false
}
