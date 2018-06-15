package apidocs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/go-openapi/spec"
)

type Page struct {
	s          *spec.Swagger
	Operations []operation
}

type Pages map[string]Page

func (page Page) Title() string {
	return page.Operations[0].gvk.Version + "." + page.Operations[0].gvk.Kind
}

func (page Page) Description() (string, error) {
	for _, def := range page.s.Definitions {
		for _, defgvk := range GroupVersionKinds(def) {
			if defgvk == page.Operations[0].gvk {
				return def.Description, nil
			}
		}
	}

	return "", fmt.Errorf("definition for %s not found", page.Operations[0].gvk)
}

func (page Page) Schema() ([]*line, error) {
	for defName, def := range page.s.Definitions {
		for _, defgvk := range GroupVersionKinds(def) {
			if defgvk == page.Operations[0].gvk {
				return buildLines(page.s, page.s.Definitions[defName], ""), nil
			}
		}
	}

	return nil, fmt.Errorf("definition for %s not found", page.Operations[0].gvk)
}

func (page Page) OutputPath() string {
	gvk := page.Operations[0].gvk

	parts := strings.Split(strings.Trim(page.Operations[0].PathName, "/"), "/")

	dir := parts[0]
	if gvk.Group == "extensions" &&
		gvk.Version == "v1" &&
		gvk.Kind == "ReplicationController" {
		dir = "api"
	}

	if gvk.Group != "" {
		dir += "-" + gvk.Group
	}

	return filepath.Join(dir, gvk.Version+"."+gvk.Kind+".adoc")
}

func (page Page) ParentTopicName() string {
	gvk := page.Operations[0].gvk

	parts := strings.Split(strings.Trim(page.Operations[0].PathName, "/"), "/")

	dir := parts[0]
	if gvk.Group == "extensions" &&
		gvk.Version == "v1" &&
		gvk.Kind == "ReplicationController" {
		dir = "api"
	}

	if gvk.Group != "" {
		dir += "/" + gvk.Group
	}
	return "/" + dir + "/" + gvk.Version
}

func BuildPages(s *spec.Swagger) (Pages, error) {
	pages := make(Pages, len(s.Paths.Paths))

	for pathName, path := range s.Paths.Paths {
		if strings.HasPrefix(pathName, "/api/v1/proxy/") { // deprecated API which requires additional logic, ignore it at least for now
			continue
		}

		for opName, op := range Operations(path) {
			o := operation{
				s:         s,
				Path:      path,
				PathName:  pathName,
				Operation: op,
				OpName:    opName,
			}

			var err error
			o.gvk, err = o.GVK()
			if err != nil {
				return nil, err
			}

			page, found := pages[o.gvk.String()]
			if !found {
				page = Page{s: s}
			}
			page.Operations = append(page.Operations, o)

			pages[o.gvk.String()] = page
		}
	}

	for key, page := range pages {
		sort.Sort(byPathName(page.Operations))
		sort.Stable(byNamespaced(page.Operations))
		sort.Stable(byPlural(page.Operations))
		sort.Stable(byOperationVerb(page.Operations))
		sort.Stable(bySubresource(page.Operations))
		sort.Stable(byProxy(page.Operations))

		pages[key] = page
	}

	return pages, nil
}

func (pages Pages) Write(root string) error {
	t, err := template.New("page.adoc").Funcs(template.FuncMap{
		"FriendlyTypeName": FriendlyTypeName,
		"EscapeMediaTypes": EscapeMediaTypes,
	}).ParseGlob("tools/genapidocs/apidocs/templates/*")
	if err != nil {
		return err
	}

	for _, page := range pages {
		path := filepath.Join(root, page.OutputPath())

		err = os.MkdirAll(filepath.Dir(path), 0777)
		if err != nil {
			return err
		}

		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()

		err = t.Execute(f, page)
		if err != nil {
			return err
		}
	}

	return nil
}
