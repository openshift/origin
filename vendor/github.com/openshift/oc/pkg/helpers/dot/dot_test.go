package dot

import "testing"

func TestQuote(t *testing.T) {
	for _, tt := range []struct {
		id       string
		expected string
	}{
		{`test`, `"test"`},
		{``, `""`},
		{`test-name`, `"test-name"`},
		{`test"`, `"test\""`},
		{`lots"of"quotes"in"this`, `"lots\"of\"quotes\"in\"this"`},
		{`"""`, `"\"\"\""`},
		{`""a"`, `"\"\"a\""`},
		{`0-"name`, `"0-\"name"`},
		{`"project"`, `"\"project\""`},
		{`foo\`, `"foo\"`},
	} {
		actual := Quote(tt.id)
		if actual != tt.expected {
			t.Errorf("Quote(%s): expected %s, actual %s", tt.id, tt.expected, actual)
		}
	}
}
