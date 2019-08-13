package raven

type causer interface {
	Cause() error
}

type errWrappedWithExtra struct {
	err       error
	extraInfo map[string]interface{}
}

func (ewx *errWrappedWithExtra) Error() string {
	if ewx.err == nil {
		return ""
	}

	return ewx.err.Error()
}

func (ewx *errWrappedWithExtra) Cause() error {
	return ewx.err
}

func (ewx *errWrappedWithExtra) ExtraInfo() Extra {
	return ewx.extraInfo
}

// WrapWithExtra adds extra data to an error before reporting to Sentry
func WrapWithExtra(err error, extraInfo map[string]interface{}) error {
	return &errWrappedWithExtra{
		err:       err,
		extraInfo: extraInfo,
	}
}

// errWithJustExtra is a regular error with just the user-provided extras added but without a cause
type errWithJustExtra interface {
	error
	ExtraInfo() Extra
}

// ErrWithExtra links Error with attached user-provided extras that will be reported alongside the Error
type ErrWithExtra interface {
	errWithJustExtra
	Cause() error
}

// Iteratively fetches all the Extra data added to an error,
// and it's underlying errors. Extra data defined first is
// respected, and is not overridden when extracting.
func extractExtra(err error) Extra {
	extra := Extra{}

	currentErr := err
	for currentErr != nil {
		if errWithExtra, ok := currentErr.(errWithJustExtra); ok {
			for k, v := range errWithExtra.ExtraInfo() {
				extra[k] = v
			}
		}

		if errWithCause, ok := currentErr.(causer); ok {
			currentErr = errWithCause.Cause()
		} else {
			currentErr = nil
		}
	}

	return extra
}
