package template

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
)

func TestNewTemplate(t *testing.T) {
	var template Template

	jsonData, _ := ioutil.ReadFile("example/project.json")
	if err := json.Unmarshal(jsonData, &template); err != nil {
		t.Errorf("Unable to process the JSON template file: %v", err)
	}
}

func TestCustomParameter(t *testing.T) {
	var template Template

	jsonData, _ := ioutil.ReadFile("example/project.json")
	json.Unmarshal(jsonData, &template)

	AddCustomTemplateParameter(Parameter{Name: "CUSTOM_PARAM", Value: "1"}, &template)
	AddCustomTemplateParameter(Parameter{Name: "CUSTOM_PARAM", Value: "2"}, &template)

	if p := GetTemplateParameterByName("CUSTOM_PARAM", &template); p == nil {
		t.Errorf("Unable to add a custom parameter to the template")
	} else {
		if p.Value != "2" {
			t.Errorf("Unable to replace the custom parameter value in template")
		}
	}

}

func ExampleProcessTemplateParameters() {
	var template Template

	jsonData, _ := ioutil.ReadFile("example/project.json")
	json.Unmarshal(jsonData, &template)

	// Define custom parameter for transformation:
	customParam := Parameter{Name: "CUSTOM_PARAM1", Value: "1"}
	AddCustomTemplateParameter(customParam, &template)

	// Generate parameter values
	GenerateParameterValues(&template, rand.New(rand.NewSource(1337)))

	// Substitute parameters with values in container env vars
	ProcessEnvParameters(&template)

	result, _ := json.Marshal(template)
	fmt.Println(string(result))
	// Output:
	// {"id":"example1","creationTimestamp":null,"buildConfigs":[{"name":"mfojtik/nginx-php-app","type":"docker","sourceUri":"https://raw.githubusercontent.com/mfojtik/phpapp/master/Dockerfile","imageRepository":"mfojtik/nginx-php-app"},{"name":"postgres","type":"docker","sourceUri":"https://raw.githubusercontent.com/docker-library/postgres/docker/9.2/Dockerfile","imageRepository":"postgres"}],"imageRepositories":[{"creationTimestamp":null,"name":"mfojtik/nginx-php-app","url":"internal.registry.com:5000/mfojtik/phpapp"},{"creationTimestamp":null,"name":"postgres","url":"registry.hub.docker.com/postgres"}],"parameters":[{"name":"DB_PASSWORD","description":"PostgreSQL admin user password","type":"string","generate":"[a-zA-Z0-9]{8}","value":"bQPdwNJi"},{"name":"DB_USER","description":"PostgreSQL username","type":"string","generate":"admin[a-zA-Z0-9]{4}","value":"adminJwWP"},{"name":"DB_NAME","description":"PostgreSQL database name","type":"string","generate":"","value":"mydb"},{"name":"REMOTE_KEY","description":"Example of remote key","type":"string","generate":"","value":"[GET:http://custom.url.int]"},{"name":"CUSTOM_PARAM1","description":"","type":"","generate":"","value":"1"}],"services":[{"kind":"Service","id":"database","creationTimestamp":null,"apiVersion":"v1beta1","port":5432,"selector":{"name":"database"},"containerPort":0},{"kind":"Service","id":"frontend","creationTimestamp":null,"apiVersion":"v1beta1","port":8080,"selector":{"name":"frontend"},"containerPort":0}],"deploymentConfigs":[{"kind":"DeploymentConfig","creationTimestamp":null,"apiVersion":"v1beta1","labels":{"name":"database"},"desiredState":{"replicas":2,"replicaSelector":{"name":"database"},"podTemplate":{"desiredState":{"manifest":{"version":"v1beta1","id":"database","volumes":null,"containers":[{"name":"postgresql","image":"postgres","ports":[{"containerPort":5432}],"env":[{"name":"PGPASSWORD","value":"bQPdwNJi"},{"name":"PGUSER","value":"adminJwWP"},{"name":"PGDATABASE","value":"mydb"},{"name":"FOO","value":"${BAR}"}]}]},"restartpolicy":{}},"labels":{"name":"database"}}}},{"kind":"DeploymentConfig","creationTimestamp":null,"apiVersion":"v1beta1","labels":{"name":"frontend"},"desiredState":{"replicas":2,"replicaSelector":{"name":"frontend"},"podTemplate":{"desiredState":{"manifest":{"version":"v1beta1","id":"frontend","volumes":null,"containers":[{"name":"frontend","image":"mfojtik/nginx-php-app","ports":[{"hostPort":8080,"containerPort":9292}],"env":[{"name":"PGPASSWORD","value":"bQPdwNJi"},{"name":"PGUSER","value":"adminJwWP"},{"name":"PGDATABASE","value":"mydb"}]}]},"restartpolicy":{}},"labels":{"name":"frontend"}}}}]}
}
