package file

import (
	"encoding/csv"
	"io"
	"os"

	"github.com/openshift/origin/pkg/auth/api"
)

type TokenAuthenticator struct {
	path   string
	tokens map[string]*api.DefaultUserInfo
}

func NewTokenAuthenticator(path string) (*TokenAuthenticator, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tokens := make(map[string]*api.DefaultUserInfo)
	reader := csv.NewReader(file)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 3 {
			continue
		}
		obj := &api.DefaultUserInfo{
			Name:  record[1],
			Scope: record[2],
		}
		if len(record) > 3 {
			obj.UID = record[3]
		}
		tokens[record[0]] = obj
	}

	return &TokenAuthenticator{
		path:   file.Name(),
		tokens: tokens,
	}, nil
}

func (a *TokenAuthenticator) AuthenticateToken(value string) (api.UserInfo, bool, error) {
	user, ok := a.tokens[value]
	if !ok {
		return nil, false, nil
	}
	return user, true, nil
}
