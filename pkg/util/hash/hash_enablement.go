package hash

type StaticHashEnablement bool

func (s StaticHashEnablement) HashOnWrite() bool {
	return bool(s)
}

type HashEnablementFunc func() bool

func (f HashEnablementFunc) HashOnWrite() bool {
	return f()
}
