package example

import (
	"fmt"
	"testing"
)

func TestExternalBinary_HelloWorld(t *testing.T) {
	fmt.Println("Hello, World!")
}

func TestExternalBinary_ShouldFail(t *testing.T) {
	t.Fatal("Oh no! This test failed.")
}
