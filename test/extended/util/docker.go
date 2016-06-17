package util

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"

	gson "encoding/json"

	dockerClient "github.com/fsouza/go-dockerclient"
	tutil "github.com/openshift/origin/test/util"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

//TagImage will apply the "tagor" tag string to the image current tagged by "tagee"
func TagImage(tagee, tagor string) error {
	client, dcerr := tutil.NewDockerClient()
	if dcerr != nil {
		return dcerr
	}
	opts := dockerClient.TagImageOptions{
		Repo:  tagee,
		Tag:   "latest",
		Force: true,
	}
	return client.TagImage(tagor, opts)
}

//InspectImage initiates the equivalent of a `docker inspect` for the "name" parameter
func InspectImage(name string) (*dockerClient.Image, error) {
	client, err := tutil.NewDockerClient()
	if err != nil {
		return nil, err
	}
	return client.InspectImage(name)
}

//PullImage initiates the equivalent of a `docker pull` for the "name" parameter
func PullImage(name string, authCfg dockerClient.AuthConfiguration) error {
	client, err := tutil.NewDockerClient()
	if err != nil {
		return err
	}
	opts := dockerClient.PullImageOptions{
		Repository: name,
		Tag:        "latest",
	}
	return client.PullImage(opts, authCfg)
}

//PushImage initiates the equivalent of a `docker push` for the "name" parameter to the local registry
func PushImage(name string, authCfg dockerClient.AuthConfiguration) error {
	client, err := tutil.NewDockerClient()
	if err != nil {
		return err
	}
	opts := dockerClient.PushImageOptions{
		Name: name,
		Tag:  "latest",
	}
	return client.PushImage(opts, authCfg)
}

//ListImages initiates the equivalent of a `docker images`
func ListImages() ([]string, error) {
	client, err := tutil.NewDockerClient()
	if err != nil {
		return nil, err
	}
	imageList, err := client.ListImages(dockerClient.ListImagesOptions{})
	if err != nil {
		return nil, err
	}
	returnIds := make([]string, 0)
	for _, image := range imageList {
		for _, tag := range image.RepoTags {
			returnIds = append(returnIds, tag)
		}
	}
	return returnIds, nil
}

//BuildAuthConfiguration constructs a non-standard dockerClient.AuthConfiguration that can be used to communicate with the openshift internal docker registry
func BuildAuthConfiguration(credKey string, oc *CLI) (*dockerClient.AuthConfiguration, error) {
	authCfg := &dockerClient.AuthConfiguration{}
	secretList, err := oc.AdminKubeREST().Secrets(oc.Namespace()).List(kapi.ListOptions{})

	g.By(fmt.Sprintf("get secret list err %v ", err))
	if err == nil {
		for _, secret := range secretList.Items {
			g.By(fmt.Sprintf("secret name %s ", secret.ObjectMeta.Name))
			if strings.Contains(secret.ObjectMeta.Name, "builder-dockercfg") {
				dockercfgToken := secret.Data[".dockercfg"]
				dockercfgTokenJson := string(dockercfgToken)
				g.By(fmt.Sprintf("docker cfg token json %s ", dockercfgTokenJson))

				creds := credentialprovider.DockerConfig{}
				err = gson.Unmarshal(dockercfgToken, &creds)
				g.By(fmt.Sprintf("json unmarshal err %v ", err))
				if err == nil {

					// borrowed from openshift/origin/pkg/build/builder/cmd/dockercfg/cfg.go, but we get the
					// secrets and dockercfg data via `oc` vs. internal use of env vars and local file reading,
					// so we don't use the public methods present there
					keyring := credentialprovider.BasicDockerKeyring{}
					keyring.Add(creds)
					authConfs, found := keyring.Lookup(credKey)
					g.By(fmt.Sprintf("found auth %v with auth cfg len %d ", found, len(authConfs)))
					if !found || len(authConfs) == 0 {
						return authCfg, err
					}
					// have seen this does not get set
					if len(authConfs[0].ServerAddress) == 0 {
						authConfs[0].ServerAddress = credKey
					}
					g.By(fmt.Sprintf("dockercfg with svrAddr %s user %s pass %s email %s ", authConfs[0].ServerAddress, authConfs[0].Username, authConfs[0].Password, authConfs[0].Email))
					c := dockerClient.AuthConfiguration{Username: authConfs[0].Username, ServerAddress: authConfs[0].ServerAddress, Password: authConfs[0].Password}
					return &c, err
				}
			}
		}
	}
	return authCfg, err
}

type MissingTagError struct {
	Tags []string
}

func (mte MissingTagError) Error() string {
	return fmt.Sprintf("the tag %s passed in was invalid, and not found in the list of images returned from docker", mte.Tags)
}

//GetImageIDForTags will obtain the hexadecimal IDs for the array of human readible image tags IDs provided
func GetImageIDForTags(comps []string) ([]string, error) {
	client, dcerr := tutil.NewDockerClient()
	if dcerr != nil {
		return nil, dcerr
	}
	imageList, serr := client.ListImages(dockerClient.ListImagesOptions{})
	if serr != nil {
		return nil, serr
	}

	returnTags := make([]string, 0)
	missingTags := make([]string, 0)
	for _, comp := range comps {
		var found bool
		for _, image := range imageList {
			for _, repTag := range image.RepoTags {
				if repTag == comp {
					found = true
					returnTags = append(returnTags, image.ID)
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			returnTags = append(returnTags, "")
			missingTags = append(missingTags, comp)
		}
	}

	if len(missingTags) == 0 {
		return returnTags, nil
	} else {
		mte := MissingTagError{
			Tags: missingTags,
		}
		return returnTags, mte
	}
}
