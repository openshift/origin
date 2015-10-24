package app

import (
	"fmt"
	"testing"
)

type mockSearcher struct {
	numResults int
}

func (m mockSearcher) Search(terms ...string) (ComponentMatches, error) {
	results := ComponentMatches{}
	for i := 0; i < m.numResults; i++ {
		results = append(results, &ComponentMatch{Argument: fmt.Sprintf("match%d", i), Score: 0.0})
	}

	return results, nil
}

func TestWeightedResolvers(t *testing.T) {
	resolver1 := WeightedResolver{mockSearcher{2}, 1.0}
	resolver2 := WeightedResolver{mockSearcher{3}, 1.0}
	wr := WeightedResolvers{resolver1, resolver2}

	_, err := wr.Resolve("image")
	if err == nil {
		t.Error("expected a multiple match error, got no error")
	}
	if _, ok := err.(ErrMultipleMatches); !ok {
		t.Errorf("expected a multiple match error, got error %v instead", err)
	}
	multiError := err.(ErrMultipleMatches)
	if len(multiError.Matches) != 5 {
		t.Errorf("expected %v matches, got %v", 5, len(multiError.Matches))
	}
}
