package container

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// AuthInfo returns the error info
type AuthInfo struct {
	Authorization string `json:"authorization"`
}

// TagInfo returns the images tag info
type TagInfo struct {
	Name           string `json:"name"`
	Reversion      bool   `json:"reversion"`
	StartTs        int64  `json:"start_ts"`
	EndTs          int64  `json:"end_ts"`
	ManifestDigest string `json:"manifest_digest"`
	ImageID        string `json:"image_id"`
	LastModified   string `json:"last_modified"`
	Expiration     string `json:"expiration"`
	DockerImageID  string `json:"docker_image_id"`
	IsManifestList bool   `json:"is_manifest_list"`
	Size           int64  `json:"size"`
}

// TagsResult returns the images tag info
type TagsResult struct {
	HasAdditional bool      `json:"has_additional"`
	Page          int       `json:"page"`
	Tags          []TagInfo `json:"tags"`
}

// QuayCLI provides function to run the quay command
type QuayCLI struct {
	EndPointPre   string
	Authorization string
}

// NewQuayCLI initialize the quay api
func NewQuayCLI() *QuayCLI {
	newclient := &QuayCLI{}
	newclient.EndPointPre = "https://quay.io/api/v1/repository/"
	authString := ""
	authFilepath := ""
	if strings.Compare(os.Getenv("QUAY_AUTH_FILE"), "") != 0 {
		authFilepath = os.Getenv("QUAY_AUTH_FILE")
	} else {
		authFilepath = "/home/cloud-user/.docker/auto/quay_auth.json"
	}
	if _, err := os.Stat(authFilepath); os.IsNotExist(err) {
		e2e.Logf("Quay auth file does not exist")
	} else {
		content, err := ioutil.ReadFile(authFilepath)
		if err != nil {
			e2e.Logf("File reading error")
		} else {
			var authJSON AuthInfo
			if err := json.Unmarshal(content, &authJSON); err != nil {
				e2e.Logf("parser json error")
			} else {
				authString = "Bearer " + authJSON.Authorization
			}
		}
	}
	if strings.Compare(os.Getenv("QUAY_AUTH"), "") != 0 {
		e2e.Logf("get quay auth from env QUAY_AUTH")
		authString = "Bearer " + os.Getenv("QUAY_AUTH")
	}
	if strings.Compare(authString, "Bearer ") == 0 {
		e2e.Failf("get quay auth failed!")
	}
	newclient.Authorization = authString
	return newclient
}

// TryDeleteTag will delete the image
func (c *QuayCLI) TryDeleteTag(imageIndex string) (bool, error) {
	if strings.Contains(imageIndex, ":") {
		imageIndex = strings.Replace(imageIndex, ":", "/tag/", 1)
	}
	endpoint := c.EndPointPre + imageIndex
	e2e.Logf("endpoint is %s", endpoint)

	client := &http.Client{}
	reqest, err := http.NewRequest("DELETE", endpoint, nil)
	if strings.Compare(c.Authorization, "") != 0 {
		reqest.Header.Add("Authorization", c.Authorization)
	}

	if err != nil {
		return false, err
	}
	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return false, err
	}
	if response.StatusCode != 204 {
		e2e.Logf("delete %s failed, response code is %d", imageIndex, response.StatusCode)
		return false, nil
	}
	return true, nil
}

// DeleteTag will delete the image
func (c *QuayCLI) DeleteTag(imageIndex string) (bool, error) {
	rc, error := c.TryDeleteTag(imageIndex)
	if rc != true {
		e2e.Logf("try to delete %s again", imageIndex)
		rc, error = c.TryDeleteTag(imageIndex)
		if rc != true {
			e2e.Failf("delete tag failed on quay.io")
		}
	}
	return rc, error
}

// CheckTagNotExist check the image exist
func (c *QuayCLI) CheckTagNotExist(imageIndex string) (bool, error) {
	if strings.Contains(imageIndex, ":") {
		imageIndex = strings.Replace(imageIndex, ":", "/tag/", 1)
	}
	endpoint := c.EndPointPre + imageIndex + "/images"
	e2e.Logf("endpoint is %s", endpoint)

	client := &http.Client{}
	reqest, err := http.NewRequest("GET", endpoint, nil)
	reqest.Header.Add("Authorization", c.Authorization)

	if err != nil {
		return false, err
	}
	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return false, err
	}
	if response.StatusCode == 404 {
		e2e.Logf("tag %s not exist", imageIndex)
		return true, nil
	}
	contents, _ := ioutil.ReadAll(response.Body)
	e2e.Logf("responce is %s", string(contents))
	return false, nil

}

// GetTagNameList get the tag name list in quay
func (c *QuayCLI) GetTagNameList(imageIndex string) ([]string, error) {
	var TagNameList []string
	tags, err := c.GetTags(imageIndex)
	if err != nil {
		return TagNameList, err
	}
	for _, tagIndex := range tags {
		TagNameList = append(TagNameList, tagIndex.Name)
	}
	return TagNameList, nil
}

// GetTags list the specificTag in repository
func (c *QuayCLI) GetTags(imageIndex string) ([]TagInfo, error) {

	var result []TagInfo
	var specificTag, indexRepository, endpoint string

	if strings.Contains(imageIndex, ":") {
		indexRepository = strings.Split(imageIndex, ":")[0] + "/tag"
		specificTag = strings.Split(imageIndex, ":")[1]
		// GET /api/v1/repository/{repository}/tag?specificTag={tag} #Filters the tags to the specific tag.
		endpoint = c.EndPointPre + indexRepository + "?specificTag=" + specificTag
		if specificTag == "" {
			// GET /api/v1/repository/{repository}/tag?onlyActiveTags=true #Filter to all active tags.
			endpoint = c.EndPointPre + indexRepository + "?onlyActiveTags=true"
		}
	} else if strings.Contains(imageIndex, "/tag/") {
		imageIndex = strings.Split(imageIndex, "tag/")[0] + "tag/"
		endpoint = c.EndPointPre + imageIndex
	}

	e2e.Logf("endpoint is %s", endpoint)

	client := &http.Client{}
	reqest, err := http.NewRequest("GET", endpoint, nil)
	if strings.Compare(c.Authorization, "") != 0 {
		reqest.Header.Add("Authorization", c.Authorization)
	}
	if err != nil {
		return result, err
	}
	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return result, err
	}
	e2e.Logf("%s", response.Status)
	if response.StatusCode != 200 {
		e2e.Logf("get %s failed, response code is %d", imageIndex, response.StatusCode)
		return result, fmt.Errorf("return code is %d, not 200", response.StatusCode)
	}
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return result, err
	}
	//e2e.Logf(string(contents))
	//unmarshal json file
	var TagsResultOut TagsResult
	if err := json.Unmarshal(contents, &TagsResultOut); err != nil {
		return result, err
	}
	result = TagsResultOut.Tags
	return result, nil

}

// GetImageDigest gets the ID of the specified image
func (c *QuayCLI) GetImageDigest(imageIndex string) (string, error) {

	var result string
	tags, err := c.GetTags(imageIndex)
	if err != nil {
		e2e.Logf("Can't get the digest, GetTags failed.")
		return result, err
	}
	imageTag := strings.Split(imageIndex, ":")[1]
	for image := range tags {
		if tags[image].Name == imageTag {
			result := tags[image].ManifestDigest
			return result, nil
		}
	}
	e2e.Logf("Can't get the digest, Manifest_digest not found.")
	return result, nil

}

func (c *QuayCLI) TryChangeTag(imageTag, manifestDigest string) (bool, error) {
	if strings.Contains(imageTag, ":") {
		imageTag = strings.Replace(imageTag, ":", "/tag/", 1)
	}
	endpoint := c.EndPointPre + imageTag
	e2e.Logf("endpoint is %s", endpoint)

	payload := ("{\"manifest_digest\": \"" + manifestDigest + "\"}")

	client := &http.Client{}
	request, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer([]byte(payload)))
	if strings.Compare(c.Authorization, "") != 0 {
		request.Header.Add("Authorization", c.Authorization)
	}
	request.Header.Set("Content-Type", "application/json")

	if err != nil {
		return false, err
	}
	response, err := client.Do(request)
	defer response.Body.Close()
	if err != nil {
		return false, err
	}
	if response.StatusCode != 201 {
		e2e.Logf("change %s failed, response code is %d", imageTag, response.StatusCode)
		return false, nil
	}
	return true, nil
}

// ChangeTag will change the image tag
func (c *QuayCLI) ChangeTag(imageTag, manifestDigest string) (bool, error) {
	rc, error := c.TryChangeTag(imageTag, manifestDigest)
	if rc != true {
		e2e.Logf("try to tag %s again", manifestDigest)
		rc, error = c.TryChangeTag(imageTag, manifestDigest)
		if rc != true {
			e2e.Logf("Change tag failed on quay.io")
		}
	}
	return rc, error
}
