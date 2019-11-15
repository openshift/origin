package fuzz

import "github.com/agnivade/levenshtein"

func Fuzz(data []byte) int {
	str := string(data)
	if len(str) == 0 {
		return -1
	}
	splitIndex := len(str) / 2
	s1 := str[:splitIndex]
	s2 := str[splitIndex+1:]
	levenshtein.ComputeDistance(s1, s2)
	return 1
}
