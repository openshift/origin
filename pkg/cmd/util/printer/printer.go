package printer

type Printer interface {
	Print(...interface{})
	Println(...interface{})
	Warn(...interface{})
	Warnln(...interface{})
	Error(...interface{})
	Errorln(...interface{})
	Success(...interface{})
	Successln(...interface{})
}
