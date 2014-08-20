package template

import (
	"fmt"
	"math/rand"
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

func ExampleTransformTemplate() {
	var resultTemplate Template

	template, err := NewTemplateFromFile("example/project.json")
	if err != nil {
		fmt.Printf("Unable to process example/simple.json template: %v", err)
	}

	// In this example, we want always produce the same result:
	template.Seed = rand.New(rand.NewSource(1337))

	// Define custom parameter for transformation:
	customParams := make([]Parameter, 1)
	customParams[0] = Parameter{Name: "CUSTOM_PARAM1", Value: "1"}

	TransformTemplate(template, &resultTemplate, customParams)

	result, _ := TemplateToJSON(resultTemplate)
	fmt.Println(string(result))
	// Output:
	// {"id":"example1","buildConfigs":[{"name":"mfojtik/nginx-php-app","type":"docker","sourceUri":"https://raw.githubusercontent.com/mfojtik/phpapp/master/Dockerfile","imageRepository":"mfojtik/nginx-php-app"},{"name":"postgres","type":"docker","sourceUri":"https://raw.githubusercontent.com/docker-library/postgres/docker/9.2/Dockerfile","imageRepository":"postgres"}],"imageRepositories":[{"name":"mfojtik/nginx-php-app","url":"internal.registry.com:5000/mfojtik/phpapp"},{"name":"postgres","url":"registry.hub.docker.com/postgres"}],"parameters":[{"name":"DB_PASSWORD","description":"PostgreSQL admin user password","type":"string","generate":"[a-zA-Z0-9]{8}","value":"bQPdwNJi","Seed":{}},{"name":"DB_USER","description":"PostgreSQL username","type":"string","generate":"admin[a-zA-Z0-9]{4}","value":"adminJwWP","Seed":{}},{"name":"DB_NAME","description":"PostgreSQL database name","type":"string","generate":"","value":"mydb","Seed":{}},{"name":"REMOTE_KEY","description":"Example of remote key","type":"string","generate":"","value":"[GET:http://custom.url.int]","Seed":{}},{"name":"CUSTOM_PARAM1","description":"","type":"","generate":"","value":"1","Seed":{}}],"services":[{"kind":"Service","id":"database","apiVersion":"v1beta1","port":5432,"selector":{"name":"database"},"containerPort":0},{"kind":"Service","id":"frontend","apiVersion":"v1beta1","port":8080,"selector":{"name":"frontend"},"containerPort":0}],"deploymentConfigs":[{"kind":"DeploymentConfig","apiVersion":"v1beta1","labels":{"name":"database"},"desiredState":{"replicas":2,"replicaSelector":{"name":"database"},"podTemplate":{"desiredState":{"manifest":{"version":"v1beta1","id":"database","volumes":null,"containers":[{"name":"postgresql","image":"postgres","ports":[{"containerPort":5432}],"env":[{"name":"PGPASSWORD","value":"bQPdwNJi"},{"name":"PGUSER","value":"adminJwWP"},{"name":"PGDATABASE","value":"mydb"},{"name":"FOO","value":"${BAR}"}]}]},"restartpolicy":{}},"labels":{"name":"database"}}}},{"kind":"DeploymentConfig","apiVersion":"v1beta1","labels":{"name":"frontend"},"desiredState":{"replicas":2,"replicaSelector":{"name":"frontend"},"podTemplate":{"desiredState":{"manifest":{"version":"v1beta1","id":"frontend","volumes":null,"containers":[{"name":"frontend","image":"mfojtik/nginx-php-app","ports":[{"hostPort":8080,"containerPort":9292}],"env":[{"name":"PGPASSWORD","value":"bQPdwNJi"},{"name":"PGUSER","value":"adminJwWP"},{"name":"PGDATABASE","value":"mydb"}]}]},"restartpolicy":{}},"labels":{"name":"frontend"}}}}],"Seed":{}}

}
