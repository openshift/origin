package backoff

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestIncremental(t *testing.T) {
	const duration = time.Millisecond
	const increment = time.Nanosecond

	algorithm := Incremental(duration, increment)

	for i := uint(0); i < 10; i++ {
		result := algorithm(i)
		expected := duration + (increment * time.Duration(i))

		if result != expected {
			t.Errorf("algorithm expected to return a %s duration, but received %s instead", expected, result)
		}
	}
}

func TestLinear(t *testing.T) {
	const duration = time.Millisecond

	algorithm := Linear(duration)

	for i := uint(0); i < 10; i++ {
		result := algorithm(i)
		expected := duration * time.Duration(i)

		if result != expected {
			t.Errorf("algorithm expected to return a %s duration, but received %s instead", expected, result)
		}
	}
}

func TestExponential(t *testing.T) {
	const duration = time.Second
	const base = 3

	algorithm := Exponential(duration, base)

	for i := uint(0); i < 10; i++ {
		result := algorithm(i)
		expected := duration * time.Duration(math.Pow(base, float64(i)))

		if result != expected {
			t.Errorf("algorithm expected to return a %s duration, but received %s instead", expected, result)
		}
	}
}

func TestBinaryExponential(t *testing.T) {
	const duration = time.Second

	algorithm := BinaryExponential(duration)

	for i := uint(0); i < 10; i++ {
		result := algorithm(i)
		expected := duration * time.Duration(math.Pow(2, float64(i)))

		if result != expected {
			t.Errorf("algorithm expected to return a %s duration, but received %s instead", expected, result)
		}
	}
}

func TestFibonacci(t *testing.T) {
	const duration = time.Millisecond

	algorithm := Fibonacci(duration)

	for i := uint(0); i < 10; i++ {
		result := algorithm(i)
		expected := duration * time.Duration(fibonacciNumber(i))

		if result != expected {
			t.Errorf("algorithm expected to return a %s duration, but received %s instead", expected, result)
		}
	}
}

func TestFibonacciNumber(t *testing.T) {
	// Fibonacci sequence
	expectedSequence := []uint{0, 1, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233}

	for i, expected := range expectedSequence {
		result := fibonacciNumber(uint(i))

		if result != expected {
			t.Errorf("fibonacci %d number expected %d, but got %d", i, expected, result)
		}
	}
}

func ExampleIncremental() {
	algorithm := Incremental(15*time.Millisecond, 10*time.Millisecond)

	for i := uint(1); i <= 5; i++ {
		duration := algorithm(i)

		fmt.Printf("#%d attempt: %s\n", i, duration)
		// Output:
		// #1 attempt: 25ms
		// #2 attempt: 35ms
		// #3 attempt: 45ms
		// #4 attempt: 55ms
		// #5 attempt: 65ms
	}
}

func ExampleLinear() {
	algorithm := Linear(15 * time.Millisecond)

	for i := uint(1); i <= 5; i++ {
		duration := algorithm(i)

		fmt.Printf("#%d attempt: %s\n", i, duration)
		// Output:
		// #1 attempt: 15ms
		// #2 attempt: 30ms
		// #3 attempt: 45ms
		// #4 attempt: 60ms
		// #5 attempt: 75ms
	}
}

func ExampleExponential() {
	algorithm := Exponential(15*time.Millisecond, 3)

	for i := uint(1); i <= 5; i++ {
		duration := algorithm(i)

		fmt.Printf("#%d attempt: %s\n", i, duration)
		// Output:
		// #1 attempt: 45ms
		// #2 attempt: 135ms
		// #3 attempt: 405ms
		// #4 attempt: 1.215s
		// #5 attempt: 3.645s
	}
}

func ExampleBinaryExponential() {
	algorithm := BinaryExponential(15 * time.Millisecond)

	for i := uint(1); i <= 5; i++ {
		duration := algorithm(i)

		fmt.Printf("#%d attempt: %s\n", i, duration)
		// Output:
		// #1 attempt: 30ms
		// #2 attempt: 60ms
		// #3 attempt: 120ms
		// #4 attempt: 240ms
		// #5 attempt: 480ms
	}
}

func ExampleFibonacci() {
	algorithm := Fibonacci(15 * time.Millisecond)

	for i := uint(1); i <= 5; i++ {
		duration := algorithm(i)

		fmt.Printf("#%d attempt: %s\n", i, duration)
		// Output:
		// #1 attempt: 15ms
		// #2 attempt: 15ms
		// #3 attempt: 30ms
		// #4 attempt: 45ms
		// #5 attempt: 75ms
	}
}
