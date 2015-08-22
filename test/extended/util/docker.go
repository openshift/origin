package util

import (
	"fmt"
	dockerClient "github.com/fsouza/go-dockerclient"
	tutil "github.com/openshift/origin/test/util"
)

//TagImage, as the name implies, will apply the "tagor" tag string to the image current tagged by "tagee"
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

//PullImage, as the name implies, initiates the equivalent of a `docker pull` for the "name" parameter
func PullImage(name string) error {
	client, err := tutil.NewDockerClient()
	if err != nil {
		return err
	}
	opts := dockerClient.PullImageOptions{
		Repository: name,
		Tag:        "latest",
	}
	return client.PullImage(opts, dockerClient.AuthConfiguration{})
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
