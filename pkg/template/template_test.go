package template

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

var sampleTemplate *Template

func TestNewTemplate(t *testing.T) {
	var err error
	sampleTemplate, err = NewTemplateFromFile("example/project.json")
	if err != nil {
		t.Errorf("Unable to process the JSON template: %v", err)
	}
}

func TestProcessParameters(t *testing.T) {
	sampleTemplate.ProcessParameters([]Parameter{})

	for _, p := range sampleTemplate.Parameters {
		if p.Value == "" {
			t.Errorf("Failed to process '%s' parameter", p.Name)
		}
		fmt.Printf("%s -> %s = %s\n", p.Name, p.Generate, p.Value)
	}
}

func TestProcessTemplate(t *testing.T) {
	sampleTemplate.Process()

	for _, c := range sampleTemplate.Containers() {
		for _, e := range c.Env {
			if strings.Contains(string(e.Value), "${") {
				if e.Name != "FOO" {
					t.Errorf("Failed to substitute %s environment variable: %s", e.Name, e.Value)
				}
			}
			fmt.Printf("%s=%s\n", e.Name, e.Value)
		}
	}

	for _, s := range sampleTemplate.ServiceLinks {
		for _, e := range s.Export {
			if strings.Contains(string(e.Value), "${") {
				if e.Name != "FOO" {
					t.Errorf("Failed to substitute %s environment variable: %s", e.Name, e.Value)
				}
			}
			fmt.Printf("%s=%s\n", e.Name, e.Value)
		}
	}
}

func ExampleTemplate_Transform() {
	template, err := NewTemplateFromFile("example/simple.json")
	if err != nil {
		fmt.Printf("Unable to process example/simple.json template: %v", err)
	}

	customParams := make([]Parameter, 3)
	customParams[0] = Parameter{Name: "CUSTOM_PARAM1", Value: "1"}
	customParams[1] = Parameter{Name: "CUSTOM_PARAM2", Value: "2"}
	customParams[2] = Parameter{Name: "CUSTOM_PARAM3", Value: "3"}

	// In this example, we want always produce the same result:
	//
	template.Seed = rand.New(rand.NewSource(1337))

	result, _ := template.Transform(customParams)

	fmt.Println(string(result))
	// Output:
	// {"id":"example2","buildConfig":null,"imageRepository":null,"parameters":[{"name":"DB_PASSWORD","description":"PostgreSQL admin user password","type":"string","generate":"[a-zA-Z0-9]{8}","value":"bQPdwNJi","Seed":{}},{"name":"DB_USER","description":"PostgreSQL username","type":"string","generate":"admin[a-zA-Z0-9]{4}","value":"adminJwWP","Seed":{}},{"name":"SAMPLE_VAR","description":"Sample","type":"string","generate":"","value":"foo","Seed":{}},{"name":"CUSTOM_PARAM1","description":"","type":"","generate":"","value":"1","Seed":{}},{"name":"CUSTOM_PARAM2","description":"","type":"","generate":"","value":"2","Seed":{}},{"name":"CUSTOM_PARAM3","description":"","type":"","generate":"","value":"3","Seed":{}}],"serviceLinks":[{"from":"database","to":"frontend","export":[{"name":"POSTGRES_ADMIN_USERNAME","value":"adminJwWP"},{"name":"POSTGRES_ADMIN_PASSWORD","value":"bQPdwNJi"},{"name":"POSTGRES_DATABASE_NAME","value":"${DB_NAME}"}]}],"services":[{"name":"database","description":"Standalone PostgreSQL 9.2 database service","labels":{"name":"database-service"},"deploymentConfig":{"deployment":{"podTemplate":{"containers":[{"name":"postgresql-1","image":{"name":"postgres","tag":"9.2"},"env":[{"name":"POSTGRES_ADMIN_USERNAME","value":"adminJwWP"},{"name":"POSTGRES_ADMIN_PASSWORD","value":"bQPdwNJi"},{"name":"FOO","value":"1"}],"ports":[{"containerPort":5432,"hostPort":5432}]}],"replicas":0}}}}],"Seed":{}}
}
