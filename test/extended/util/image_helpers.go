package util

import (
	"fmt"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"time"
)

//CorruptImage is a helper that tags the image to be corrupted, the corruptee, as the corruptor string, resulting in the wrong image being used when corruptee is referenced later on;  strategy is for ginkgo debug; ginkgo error checking leveraged
func CorruptImage(corruptee, corruptor, strategy string) {
	g.By(fmt.Sprintf("\n%s  Calling docker tag to corrupt %s builder image %s by tagging %s", time.Now().Format(time.RFC850), strategy, corruptee, corruptor))

	cerr := TagImage(corruptee, corruptor)

	g.By(fmt.Sprintf("\n%s  Tagging %s to %s complete with err %v", time.Now().Format(time.RFC850), corruptor, corruptee, cerr))
	o.Expect(cerr).NotTo(o.HaveOccurred())
}

//ResetImage is a helper the allows the programmer to undo any corruption performed by CorruptImage; ginkgo error checking leveraged
func ResetImage(tags map[string]string) {
	g.By(fmt.Sprintf("\n%s Calling docker tag to reset images", time.Now().Format(time.RFC850)))

	for corruptedTag, goodTag := range tags {
		err := TagImage(corruptedTag, goodTag)
		g.By(fmt.Sprintf("\n%s  Reset for %s to %s complete with err %v", time.Now().Format(time.RFC850), corruptedTag, goodTag, err))
		o.Expect(err).NotTo(o.HaveOccurred())
	}

}

//VerifyImagesSame will take the two supplied image tags and see if they reference the same hexadecimal image ID;  strategy is for debug
func VerifyImagesSame(comp1, comp2, strategy string) {
	tag1 := comp1 + ":latest"
	tag2 := comp2 + ":latest"

	comps := []string{tag1, tag2}
	retIDs, gerr := GetImageIDForTags(comps)

	o.Expect(gerr).NotTo(o.HaveOccurred())
	g.By(fmt.Sprintf("\n%s %s  compare image - %s, %s, %s, %s", time.Now().Format(time.RFC850), strategy, tag1, tag2, retIDs[0], retIDs[1]))
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
	g.By(fmt.Sprintf("\n%s %s  compare image - %s, %s, %s, %s", time.Now().Format(time.RFC850), strategy, tag1, tag2, retIDs[0], retIDs[1]))
	o.Ω(len(retIDs[0])).Should(o.BeNumerically(">", 0))
	o.Ω(len(retIDs[1])).Should(o.BeNumerically(">", 0))
	o.Ω(retIDs[0] != retIDs[1]).Should(o.BeTrue())
}

//WaitForBuild is a wrapper for WaitForABuild in this package that takes in an oc/cli client; some ginkgo based debug along with ginkgo error checking
func WaitForBuild(context, buildName string, oc *CLI) {
	g.By(fmt.Sprintf("\n%s %s:   waiting for %s", time.Now().Format(time.RFC850), context, buildName))
	WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, CheckBuildSuccessFunc, CheckBuildFailedFunc)
	// do not care if build returned an error ... entirely possible ... we only check if the image was updated or left the same appropriately
	g.By(fmt.Sprintf("\n%s %s   done waiting for %s", time.Now().Format(time.RFC850), context, buildName))
}

//StartBuild creates a build config from the supplied json file (not a template) and then starts a build, using the supplied oc/cli client for both operations; ginkgo error checking included
func StartBuild(jsonFile, buildPrefix string, oc *CLI) {
	err := oc.Run("create").Args("-f", jsonFile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	_, berr := oc.Run("start-build").Args(buildPrefix).Output()
	o.Expect(berr).NotTo(o.HaveOccurred())
}
