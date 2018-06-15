package util

import (
	"fmt"

	g "github.com/onsi/ginkgo"
)

//DumpAndReturnTagging takes and array of tags and obtains the hex image IDs, dumps them to ginkgo for printing, and then returns them
func DumpAndReturnTagging(tags []string) ([]string, error) {
	hexIDs, err := GetImageIDForTags(tags)
	if err != nil {
		return nil, err
	}
	for i, hexID := range hexIDs {
		fmt.Fprintf(g.GinkgoWriter, "tag %s hex id %s ", tags[i], hexID)
	}
	return hexIDs, nil
}

//CreateResource creates the resources from the supplied json file (not a template); ginkgo error checking included
func CreateResource(jsonFilePath string, oc *CLI) error {
	err := oc.Run("create").Args("-f", jsonFilePath).Execute()
	return err
}
