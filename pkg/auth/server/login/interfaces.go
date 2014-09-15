package login

import ()

type CSRF interface {
	Generate() (string, error)
	Check(string) (bool, error)
}
