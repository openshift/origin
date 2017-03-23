package html

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

const (
	sampleGetForm = `
<html>
<body>
    <form id="1">
        <input id="i1-1" name="a" value="av">
        <input id="i1-2" type="submit" name="sa" value="sav">
        <input id="i1-3" type="submit" name="sb" value="sbv">
    </form>
    <form id="2">
        <input id="i2-1" name="c" value="cv">
        <input id="i2-2" type="submit" name="sc" value="scv">
        <input id="i2-3" type="submit" name="sd" value="sdv">
    </form>
</body>
</html>
`

	samplePostForm = `
<html>
<body>
    <form id="1" action="/test" method="post">
        <input id="i1-1" name="a" value="av">
        <input id="i1-2" type="submit" name="sa" value="sav">
        <input id="i1-3" type="submit" name="sb" value="sbv">
    </form>
    <form id="2">
        <input id="i2-1" name="c" value="cv">
        <input id="i2-2" type="submit" name="sc" value="scv">
        <input id="i2-3" type="submit" name="sd" value="sdv">
    </form>
</body>
</html>
`
)

func TestGetElementsByTagName(t *testing.T) {
	tests := []struct {
		Data        string
		TagName     string
		ExpectedIds []string
	}{
		{
			Data:        sampleGetForm,
			TagName:     `form`,
			ExpectedIds: []string{"1", "2"},
		},
		{
			Data:        sampleGetForm,
			TagName:     `input`,
			ExpectedIds: []string{"i1-1", "i1-2", "i1-3", "i2-1", "i2-2", "i2-3"},
		},
	}

	for i, tc := range tests {
		root, err := html.Parse(strings.NewReader(tc.Data))
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}
		elements := GetElementsByTagName(root, tc.TagName)
		ids := []string{}
		for _, e := range elements {
			id, _ := GetAttr(e, "id")
			ids = append(ids, id)
		}
		if !reflect.DeepEqual(tc.ExpectedIds, ids) {
			t.Errorf("%d: expected %#v, got %#v", i, tc.ExpectedIds, ids)
			continue
		}
	}
}

func TestNewRequestFromForm(t *testing.T) {

	currentURL, _ := url.Parse("https://localhost:1234")

	relativeGetReq, _ := http.NewRequest("GET", "https://localhost:1234?a=av&sa=sav", nil)

	relativePostReq, _ := http.NewRequest("POST", "https://localhost:1234/test", strings.NewReader(url.Values{
		"a":  []string{"av"},
		"sa": []string{"sav"},
	}.Encode()))
	relativePostReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tests := []struct {
		Data            string
		CurrentURL      *url.URL
		ExpectedRequest *http.Request
	}{
		{
			Data:            sampleGetForm,
			CurrentURL:      currentURL,
			ExpectedRequest: relativeGetReq,
		},
		{
			Data:            samplePostForm,
			CurrentURL:      currentURL,
			ExpectedRequest: relativePostReq,
		},
	}

	for i, tc := range tests {
		root, err := html.Parse(strings.NewReader(tc.Data))
		if err != nil {
			t.Fatal(err)
		}
		forms := GetElementsByTagName(root, "form")
		req, err := NewRequestFromForm(forms[0], tc.CurrentURL, nil)
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(tc.ExpectedRequest, req) {
			t.Errorf("%d: expected\n%#v\ngot\n%#v", i, tc.ExpectedRequest, req)
			continue
		}
	}
}
