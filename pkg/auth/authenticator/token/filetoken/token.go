package filetoken

import (
	"encoding/csv"
	"errors"
	"io"
	"os"

	"k8s.io/kubernetes/pkg/auth/user"
)

type TokenAuthenticator struct {
	path   string
	tokens map[string]*user.DefaultInfo
}

func NewTokenAuthenticator(path string) (*TokenAuthenticator, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tokens := make(map[string]*user.DefaultInfo)
	reader := csv.NewReader(file)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 2 {
			continue
		}
		obj := &user.DefaultInfo{
			Name: record[1],
		}
		if len(record) > 2 {
			obj.UID = record[2]
		}
		tokens[record[0]] = obj
	}

	return &TokenAuthenticator{
		path:   file.Name(),
		tokens: tokens,
	}, nil
}

func (a *TokenAuthenticator) AuthenticateToken(value string) (user.Info, bool, error) {
	user, ok := a.tokens[value]
	if !ok {
		return nil, false, errors.New("Invalid token")
	}
	return user, true, nil
}
