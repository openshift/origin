package url

import (
	"fmt"
	"testing"
)

func TestTestsToScript(t *testing.T) {
	tests := []*URLTest{
		MustURLTest("GET", "https://www.google.com"),
	}
	fmt.Println(testsToScript(tests))
}
