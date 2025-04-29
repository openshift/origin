package property

import (
	"fmt"
)

type ParseError struct {
	Idx int
	Typ string
	Err error
}

func (e ParseError) Error() string {
	return fmt.Sprintf("parse property[%d] of type %q: %v", e.Idx, e.Typ, e.Err)
}

type MatchMissingError struct {
	foundType    string
	foundValue   interface{}
	expectedType string
}

func (e MatchMissingError) Error() string {
	return fmt.Sprintf("property %q for %+v requires matching %q property", e.foundType, e.foundValue, e.expectedType)
}
