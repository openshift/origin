package main

const debug = false

type marshalJSON interface {
	MarshalJSON() ([]byte, error)
}

func debugJSON(mk marshalJSON) {
	if debug {
		b, err := mk.MarshalJSON()
		if err == nil {
			println(string(b))
		}
	}
}
