package generator

import (
	"io/ioutil"
	"net/http"
	"strings"
)

func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body), err
}

func replaceUrlWithData(s *string, expresion string) error {
	result, err := httpGet(expresion[5 : len(expresion)-1])
	if err != nil {
		return err
	}
	*s = strings.Replace(*s, expresion, strings.TrimSpace(result), 1)
	return nil
}
