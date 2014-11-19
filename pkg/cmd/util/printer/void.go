package printer

type VoidPrinter struct{}

func (v *VoidPrinter) Printf(s string, i ...interface{})   {}
func (v *VoidPrinter) Warnf(s string, i ...interface{})    {}
func (v *VoidPrinter) Errorf(s string, i ...interface{})   {}
func (v *VoidPrinter) Successf(s string, i ...interface{}) {}
