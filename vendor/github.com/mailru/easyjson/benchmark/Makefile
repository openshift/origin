all:
	go test -benchmem -bench .
	go test -benchmem -tags use_ffjson -bench .
	go test -benchmem -tags use_jsoniter -bench .
	go test -benchmem -tags use_codec -bench .
