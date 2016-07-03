package scripts

import "testing"

func TestConvertEnvironment(t *testing.T) {
	env := []Environment{
		{"FOO", "BAR"},
	}
	result := ConvertEnvironment(env)
	if len(result) != 1 {
		t.Errorf("Expected 1 item, got %d", len(result))
	}
	if result[0] != "FOO=BAR" {
		t.Errorf("Expected FOO=BAR, got %v", result)
	}
}
