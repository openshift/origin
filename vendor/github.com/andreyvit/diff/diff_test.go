package diff

import (
	"testing"
)

func Test_CharacterDiff_NoChanges(t *testing.T) {
	actual := CharacterDiff("abc", "abc")
	expected := "abc"
	if actual != expected {
		t.Errorf("Got %#v instead of %#v", actual, expected)
	}
}

func Test_CharacterDiff_SomeChanges(t *testing.T) {
	actual := CharacterDiff("abc", "abdef")
	expected := "ab(~~c~~)(++def++)"
	if actual != expected {
		t.Errorf("Got %#v instead of expected %#v", actual, expected)
	}
}

func Test_LineDiff_NoChanges(t *testing.T) {
	o(t, "abc", "abc", " abc")
}

func Test_LineDiff_EverythingChanged(t *testing.T) {
	o(t, "abc", "def", "-abc\n+def")
}

func Test_LineDiff_AddedLine(t *testing.T) {
	o(t, "abc\nxyz", "abc\ndef\nxyz", " abc\n+def\n xyz")
}

func Test_LineDiff_AddedMultipleLines(t *testing.T) {
	o(t, "abc\nxyz", "abc\ndef\nghi\njkl\nxyz", " abc\n+def\n+ghi\n+jkl\n xyz")
}

func Test_LineDiff_DeletedLine(t *testing.T) {
	o(t, "abc\ndef\nxyz", "abc\nxyz", " abc\n-def\n xyz")
}

func Test_LineDiff_DeletedMultipleLines(t *testing.T) {
	o(t, "abc\ndef\nghi\njkl\nxyz", "abc\nxyz", " abc\n-def\n-ghi\n-jkl\n xyz")
}

func Test_LineDiff_ReplacedLine(t *testing.T) {
	o(t, "abc\ndef\nxyz", "abc\nghi\nxyz", " abc\n-def\n+ghi\n xyz")
}

func Test_LineDiff_ReplacedMultipleLines(t *testing.T) {
	o(t, "abc\ndef\nghi\nxyz", "abc\njkl\nnop\nxyz", " abc\n-def\n-ghi\n+jkl\n+nop\n xyz")
}

func Test_LineDiff_ReplacedPartOfLine(t *testing.T) {
	o(t, "abc\ndef\nxyz", "abc\ndxf\nxyz", " abc\n-def\n+dxf\n xyz")
}

func Test_LineDiff_ReplacedPartsOfMultipleConsecutiveLines(t *testing.T) {
	o(t, "abc\ndef\nghi\njkl\nxyz", "abc\ndxf\nghix\nxjkl\nxyz", " abc\n-def\n-ghi\n-jkl\n+dxf\n+ghix\n+xjkl\n xyz")
}

func Test_LineDiff_ReplacedTrailingPartOfLastLine(t *testing.T) {
	o(t, "abc\ndef\nxyz", "abc\ndef\nxyzuv", " abc\n def\n-xyz\n+xyzuv")
}

func o(t *testing.T, a, b, expected string) {
	actual := LineDiff(a, b)
	if actual != expected {
		t.Logf("Actual:\n%v\n\nExpected:\n%v", actual, expected)
		t.Errorf("Got %#v instead of expected %#v", actual, expected)
	}
}
