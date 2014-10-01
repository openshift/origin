package printer

type Printer interface {
	Printf(string, ...interface{})
	Warnf(string, ...interface{})
	Errorf(string, ...interface{})
	Successf(string, ...interface{})
}
