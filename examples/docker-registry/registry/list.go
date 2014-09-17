package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

// RepositoryList holds repositories return when quering for list.
type RepositoryList struct {
	Items []Repository `json:"results"`
}

// Repository holds information regarding single repository.
type Repository struct {
	Name string `json:"name"`
}

// Image holds information regarding single image.
type Image struct {
	ID   string `json:"id"`
	Size int    `json:"Size"`
}

func main() {
	host := "http://localhost:5000"
	if len(os.Args) == 2 {
		host = "http://" + os.Args[1]
	}

	// get repository list
	resp, err := http.Get(host + "/v1/search")
	if err != nil {
		fmt.Errorf("Could not reach repository under %s!", host)
		return
	}

	repoBody, _ := ioutil.ReadAll(resp.Body)
	repositoryList := &RepositoryList{}
	json.Unmarshal(repoBody, &repositoryList)

	if len(repositoryList.Items) == 0 {
		fmt.Println("No repositories found.")
		return
	}

	// iterate over repositories
	for _, repo := range repositoryList.Items {
		// get images from single repository
		resp, _ := http.Get(host + "/v1/repositories/" + repo.Name + "/images")

		// iterate over images
		imgBody, _ := ioutil.ReadAll(resp.Body)
		imageList := []Image{}
		json.Unmarshal(imgBody, &imageList)

		size := 0
		for _, img := range imageList {
			// get single image information
			resp, _ := http.Get(host + "/v1/images/" + img.ID + "/json")
			sizeBody, _ := ioutil.ReadAll(resp.Body)
			image := &Image{}
			json.Unmarshal(sizeBody, &image)
			// sum single repository images
			size += image.Size / 1048576
		}
		fmt.Printf("%s: %dMB\n", repo.Name, size)
	}
}
