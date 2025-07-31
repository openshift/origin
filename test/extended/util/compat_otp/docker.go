package compat_otp

import (
	"fmt"

	dockerClient "github.com/fsouza/go-dockerclient"
)

// ListImages initiates the equivalent of a `docker images`
func ListImages() ([]string, error) {
	client, err := dockerClient.NewClientFromEnv()
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

type MissingTagError struct {
	Tags []string
}

func (mte MissingTagError) Error() string {
	return fmt.Sprintf("the tag %s passed in was invalid, and not found in the list of images returned from docker", mte.Tags)
}

// GetImageIDForTags will obtain the hexadecimal IDs for the array of human readible image tags IDs provided
func GetImageIDForTags(comps []string) ([]string, error) {
	client, dcerr := dockerClient.NewClientFromEnv()
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
