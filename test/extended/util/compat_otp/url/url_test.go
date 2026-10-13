package url

import (
	"fmt"
	"testing"
)

func TestTestsToScript(t *testing.T) {
	tests := []*Test{
		Expect("GET", "https://www.google.com"),
	}
	fmt.Println(testsToScript(tests))
}
