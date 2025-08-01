package rosacli

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func ParseLabels(labels string) []string {
	return ParseCommaSeparatedStrings(labels)
}

func ParseTaints(taints string) []string {
	return ParseCommaSeparatedStrings(taints)
}

func ParseTuningConfigs(tuningConfigs string) []string {
	return ParseCommaSeparatedStrings(tuningConfigs)
}

func ParseCommaSeparatedStrings(input string) (output []string) {
	split := strings.Split(strings.ReplaceAll(input, " ", ""), ",")
	for _, item := range split {
		if strings.TrimSpace(item) != "" {
			output = append(output, item)
		}
	}
	return
}

// Generate random string
func GenerateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())

	s := make([]byte, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func GenerateRandomName(prefix string, n int) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ToLower(GenerateRandomString(n)))
}
