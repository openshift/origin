package util

import (
	"fmt"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
)

//CorruptImage is a helper that tags the image to be corrupted, the corruptee, as the corruptor string, resulting in the wrong image being used when corruptee is referenced later on;  strategy is for ginkgo debug; ginkgo error checking leveraged
func CorruptImage(corruptee, corruptor string) {
	g.By(fmt.Sprintf("Calling docker tag to corrupt builder image %s by tagging %s", corruptee, corruptor))

	cerr := TagImage(corruptee, corruptor)

	g.By(fmt.Sprintf("Tagging %s to %s complete with err %v", corruptor, corruptee, cerr))
	o.Expect(cerr).NotTo(o.HaveOccurred())
	VerifyImagesSame(corruptee, corruptor, "image corruption")
}

//ResetImage is a helper the allows the programmer to undo any corruption performed by CorruptImage; ginkgo error checking leveraged
func ResetImage(tags map[string]string) {
	g.By(fmt.Sprintf("Calling docker tag to reset images"))

	for corruptedTag, goodTag := range tags {
		err := TagImage(corruptedTag, goodTag)
		g.By(fmt.Sprintf("Reset for %s to %s complete with err %v", corruptedTag, goodTag, err))
		o.Expect(err).NotTo(o.HaveOccurred())
	}

}

//DumpAndReturnTagging takes and array of tags and obtains the hex image IDs, dumps them to ginkgo for printing, and then returns them
func DumpAndReturnTagging(tags []string) []string {
	hexIDs, err := GetImageIDForTags(tags)
	o.Expect(err).NotTo(o.HaveOccurred())
	for i, hexID := range hexIDs {
		g.By(fmt.Sprintf("tag %s hex id %s ", tags[i], hexID))
	}
	return hexIDs
}

//VerifyImagesSame will take the two supplied image tags and see if they reference the same hexadecimal image ID;  strategy is for debug
func VerifyImagesSame(comp1, comp2, strategy string) {
	tag1 := comp1 + ":latest"
	tag2 := comp2 + ":latest"

	comps := []string{tag1, tag2}
	retIDs, gerr := GetImageIDForTags(comps)

	o.Expect(gerr).NotTo(o.HaveOccurred())
	g.By(fmt.Sprintf("%s  compare image - %s, %s, %s, %s", strategy, tag1, tag2, retIDs[0], retIDs[1]))
	o.Ω(len(retIDs[0])).Should(o.BeNumerically(">", 0))
	o.Ω(len(retIDs[1])).Should(o.BeNumerically(">", 0))
	o.Ω(retIDs[0]).Should(o.Equal(retIDs[1]))
}

//VerifyImagesDifferent will that the two supplied image tags and see if they reference different hexadecimal image IDs; strategy is for ginkgo debug, also leverage ginkgo error checking
func VerifyImagesDifferent(comp1, comp2, strategy string) {
	tag1 := comp1 + ":latest"
	tag2 := comp2 + ":latest"

	comps := []string{tag1, tag2}
	retIDs, gerr := GetImageIDForTags(comps)

	o.Expect(gerr).NotTo(o.HaveOccurred())
	g.By(fmt.Sprintf("%s  compare image - %s, %s, %s, %s", strategy, tag1, tag2, retIDs[0], retIDs[1]))
	o.Ω(len(retIDs[0])).Should(o.BeNumerically(">", 0))
	o.Ω(len(retIDs[1])).Should(o.BeNumerically(">", 0))
	o.Ω(retIDs[0] != retIDs[1]).Should(o.BeTrue())
}

//WaitForBuild is a wrapper for WaitForABuild in this package that takes in an oc/cli client; some ginkgo based debug along with ginkgo error checking
func WaitForBuild(context, buildName string, oc *CLI) {
	g.By(fmt.Sprintf("%s:   waiting for %s", context, buildName))
	WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, CheckBuildSuccessFunc, CheckBuildFailedFunc)
	// do not care if build returned an error ... entirely possible ... we only check if the image was updated or left the same appropriately
	g.By(fmt.Sprintf("%s   done waiting for %s", context, buildName))
}

//StartBuildFromJSON creates a build config from the supplied json file (not a template) and then starts a build, using the supplied oc/cli client for both operations; ginkgo error checking included
func StartBuildFromJSON(jsonFile, buildPrefix string, oc *CLI) {
	err := oc.Run("create").Args("-f", jsonFile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	_, berr := oc.Run("start-build").Args(buildPrefix).Output()
	o.Expect(berr).NotTo(o.HaveOccurred())
}

//StartBuild starts a build, with the assumption that the build config was previously created
func StartBuild(buildPrefix string, oc *CLI) {
	_, err := oc.Run("start-build").Args(buildPrefix).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//CreateResource creates the resources from the supplied json file (not a template); ginkgo error checking included
func CreateResource(jsonFilePath string, oc *CLI) error {
	err := oc.Run("create").Args("-f", jsonFilePath).Execute()
	return err
}
