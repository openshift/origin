package argv

import (
	"math"
	"testing"
)

var (
	parseText = ` a aa a'aa' a"aa"a
		 a$PATH a"$PATH" a'$PATH'
		 a"$*" a"$0" a"$\"
		 a| a|a
		 a"\A" a"\a\b\f\n\r\t\v\\\$" \t a'\A' a'\t'` +
		" a`ls /` `ls ~`"
	env = ParseEnv([]string{
		"PATH=/bin",
		"*=a",
	})
)

func TestScanner(t *testing.T) {
	gots, err := Scan(
		[]rune(parseText),
		env,
	)
	if err != nil {
		t.Fatal(err)
	}
	expects := []Token{
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("aa")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("aaa")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("aaaa")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a/bin")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a/bin")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a$PATH")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("aa")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a$\\")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a")},
		{Type: TokPipe},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a")},
		{Type: TokPipe},
		{Type: TokString, Value: []rune("a")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a\\A")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a\a\b\f\n\r\t\v\\$")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("t")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a\\A")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a\t")},
		{Type: TokSpace},
		{Type: TokString, Value: []rune("a")},
		{Type: TokReversequote, Value: []rune("ls /")},
		{Type: TokSpace},
		{Type: TokReversequote, Value: []rune("ls ~")},
		{Type: TokEOF},
	}
	if len(gots) != len(expects) {
		t.Errorf("token count is not equal: expect %d, got %d", len(expects), len(gots))
	}
	l := int(math.Min(float64(len(gots)), float64(len(expects))))
	for i := 0; i < l; i++ {
		got := gots[i]
		expect := expects[i]
		if got.Type != expect.Type {
			t.Errorf("token type is not equal: %d: expect %d, got %d", i, expect.Type, got.Type)
		}

		if expect.Type != TokSpace && string(got.Value) != string(expect.Value) {
			t.Errorf("token value is not equal: %d: expect %s, got %s", i, string(expect.Value), string(got.Value))
		}
	}

	for _, text := range []string{
		`a"`, `a'`, `a"\`, "`ls ~", `a\`,
	} {
		_, err := Scan([]rune(text), nil)
		if err != ErrInvalidSyntax {
			t.Errorf("expect unexpected eof error, but got: %v", err)
		}
	}
}
