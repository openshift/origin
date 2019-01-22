package store

import (
	"sync/atomic"
)

type (
	Getter interface {
		Get() (string, error)
	}

	GetFunc func() (string, error)

	Setter interface {
		Set(string) error
	}

	SetFunc func(string) error

	// Singleton is a thread-safe abstraction to load and store a string
	Singleton interface {
		Getter
		Setter
	}

	SingletonAdapter struct {
		GetFunc
		SetFunc
	}

	SingletonDecorator interface {
		Decorate(Singleton) Singleton
	}

	Error string
)

func (err Error) Error() string { return string(err) }

const ErrNotFound = Error("value not found in store")

func (f GetFunc) Get() (string, error) { return f() }
func (f SetFunc) Set(s string) error   { return f(s) }

func NewInMemorySingleton() Singleton {
	var value atomic.Value
	return &SingletonAdapter{
		func() (string, error) {
			x := value.Load()
			if x == nil {
				return "", ErrNotFound
			}
			return x.(string), nil
		},
		func(s string) error {
			value.Store(s)
			return nil
		},
	}
}

type (
	GetFuncDecorator func(Getter, string, error) (string, error)
	SetFuncDecorator func(Setter, string, error) error
)

func DoSet() SetFuncDecorator {
	return func(s Setter, v string, _ error) error {
		return s.Set(v)
	}
}

func (f SetFuncDecorator) AndThen(f2 SetFuncDecorator) SetFuncDecorator {
	return func(s Setter, v string, err error) error {
		err = f(s, v, err)
		if err != nil {
			return err
		}
		return f2(s, v, nil)
	}
}

func (f SetFuncDecorator) Decorate(s Singleton) Singleton {
	if f == nil {
		return s
	}
	return &SingletonAdapter{
		s.Get,
		SetFunc(func(v string) error {
			return f(s, v, nil)
		}),
	}
}

func DoGet() GetFuncDecorator {
	return func(s Getter, _ string, _ error) (string, error) {
		return s.Get()
	}
}

func (f GetFuncDecorator) AndThen(f2 GetFuncDecorator) GetFuncDecorator {
	return func(s Getter, v string, err error) (string, error) {
		v, err = f(s, v, err)
		if err != nil {
			return v, err
		}
		return f2(s, v, nil)
	}
}

func (f GetFuncDecorator) Decorate(s Singleton) Singleton {
	if f == nil {
		return s
	}
	return &SingletonAdapter{
		GetFunc(func() (string, error) {
			return f(s, "", nil)
		}),
		s.Set,
	}
}

func DecorateSingleton(s Singleton, ds ...SingletonDecorator) Singleton {
	for _, d := range ds {
		if d != nil {
			s = d.Decorate(s)
		}
	}
	return s
}

// GetOrPanic curries the result of a Getter invocation: the returned func only ever returns the string component when
// the error component of the underlying Get() call is nil. If Get() generates an error then the curried func panics.
func GetOrPanic(g Getter) func() string {
	return func() string {
		v, err := g.Get()
		if err != nil {
			panic(err)
		}
		return v
	}
}

func GetIgnoreErrors(g Getter) func() string {
	return func() string {
		v, _ := g.Get()
		return v
	}
}

// SetOrPanic curries the result of a Setter invocation: the returned func only ever returns normally when the error
// component of the underlying Set() call is nil. If Set() generates an error then the curried func panics.
func SetOrPanic(s Setter) func(v string) {
	return func(v string) {
		err := s.Set(v)
		if err != nil {
			panic(err)
		}
	}
}
