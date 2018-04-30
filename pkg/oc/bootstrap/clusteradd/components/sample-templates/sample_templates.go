package sample_templates

import (
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
)

var templateLocations = map[string]string{
	"mongodb":                    "examples/db-templates/mongodb-persistent-template.json",
	"mariadb":                    "examples/db-templates/mariadb-persistent-template.json",
	"mysql":                      "examples/db-templates/mysql-persistent-template.json",
	"postgresql":                 "examples/db-templates/postgresql-persistent-template.json",
	"cakephp quickstart":         "examples/quickstarts/cakephp-mysql-persistent.json",
	"dancer quickstart":          "examples/quickstarts/dancer-mysql-persistent.json",
	"django quickstart":          "examples/quickstarts/django-postgresql-persistent.json",
	"nodejs quickstart":          "examples/quickstarts/nodejs-mongodb-persistent.json",
	"rails quickstart":           "examples/quickstarts/rails-postgresql-persistent.json",
	"jenkins pipeline ephemeral": "examples/jenkins/jenkins-ephemeral-template.json",
	"sample pipeline":            "examples/jenkins/pipeline/samplepipeline.yaml",
}

type SampleTemplatesComponentOptions struct {
	InstallContext componentinstall.Context
}

func (c *SampleTemplatesComponentOptions) Name() string {
	return "sample-templates"
}

func (c *SampleTemplatesComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	componentsToInstall := componentinstall.Components{}
	for name, location := range templateLocations {
		componentsToInstall = append(componentsToInstall,
			componentinstall.List{
				Name:      c.Name() + "/" + name,
				Namespace: "openshift",
				List:      bootstrap.MustAsset(location),
			}.MakeReady(c.InstallContext.ClientImage(), c.InstallContext.BaseDir()),
		)
	}

	return componentsToInstall.Install(dockerClient)
}
