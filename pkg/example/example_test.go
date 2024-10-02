package example

import (
	"fmt"
	"testing"
)

var BuildTags string

func TestExternalBinary_HelloWorld(t *testing.T) {
	fmt.Println("Hello, World!")
        fmt.Println("Build Tags:", BuildTags)
}

func TestExternalBinary_ShouldFail(t *testing.T) {
	t.Fatal("Oh no! This test failed.")
}
