package printer

type VoidPrinter struct{}

func (v *VoidPrinter) Print(i ...interface{})     {}
func (v *VoidPrinter) Println(i ...interface{})   {}
func (v *VoidPrinter) Warn(i ...interface{})      {}
func (v *VoidPrinter) Warnln(i ...interface{})    {}
func (v *VoidPrinter) Error(i ...interface{})     {}
func (v *VoidPrinter) Errorln(i ...interface{})   {}
func (v *VoidPrinter) Success(i ...interface{})   {}
func (v *VoidPrinter) Successln(i ...interface{}) {}
