package types

type TestSpec interface {
	CodeLocation() []CodeLocation
	Text() string
	AppendText(text string)
}
