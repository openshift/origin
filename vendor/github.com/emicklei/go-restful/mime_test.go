package restful

import (
	"fmt"
	"testing"
)

// go test -v -test.run TestSortMimes ...restful
func TestSortMimes(t *testing.T) {
	accept := "text/html; q=0.8, text/plain, image/gif,  */*; q=0.01, image/jpeg"
	result := sortedMimes(accept)
	got := fmt.Sprintf("%v", result)
	want := "[{text/plain 1} {image/gif 1} {image/jpeg 1} {text/html 0.8} {*/* 0.01}]"
	if got != want {
		t.Errorf("bad sort order of mime types:%s", got)
	}
}

func TestNonNumberQualityInAccept_issue400(t *testing.T) {
	accept := `text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3`
	result := sortedMimes(accept)
	if got, want := len(result), 7; got != want {
		t.Errorf("got %d want %d quality mimes", got, want)
	}
}
