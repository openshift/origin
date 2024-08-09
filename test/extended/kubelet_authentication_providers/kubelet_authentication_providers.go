package kubelet_authentication_providers

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/sts"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cluster-lifecycle][Feature:KubeletAuthenticationProviders][Serial] KubeletAuthenticationProviders should", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("kubelet-authentication-providers")

	// OCP-70744 - Pull images from ECR repository should succeed
	// author: zhsun@redhat.com
	g.It("be able to pull images from ECR", func() {
		exutil.SkipIf(oc, "AWS")
		g.By("Check if user has ecr:CreateRepository permission to create ECR")
		exutil.GetAwsCredentialFromCluster(oc)
		region := exutil.GetClusterRegion(oc)
		sess := exutil.InitAwsSession(region)
		iamClient := exutil.NewIAMClient(sess)
		stsClient := exutil.NewDelegatingStsClient(sts.New(sess))
		ecrPermission, err := exutil.CheckECRPermission(iamClient, stsClient)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to check permission to create ECR")
		if !ecrPermission {
			g.Skip("Skip for this account doesn't have ecr:CreateRepository permission to create ECR!")
		}

		g.By("Add the AmazonEC2ContainerRegistryReadOnly policy")
		infrastructureName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to get infrastructureName")
		isSingleNode, err := exutil.IsSingleNode(context.Background(), oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		roleName := ""
		if isSingleNode {
			roleName = infrastructureName + "-master-role"
		} else {
			roleName = infrastructureName + "-worker-role"
		}
		rolePolicyArn := "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
		err = iamClient.AttachRolePolicy(roleName, rolePolicyArn)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to add AmazonEC2ContainerRegistryReadOnly policy")
		defer iamClient.DetachRolePolicy(roleName, rolePolicyArn)

		g.By("Create a ECR repository and get authorization token")
		registryName := "ecr-registry"
		ecrClient := exutil.NewECRClient(sess)
		repositoryUri, err := ecrClient.CreateContainerRepository(registryName)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create container registry")
		defer ecrClient.DeleteContainerRepository(registryName)
		password, _ := ecrClient.GetAuthorizationToken()
		o.Expect(password).NotTo(o.BeEmpty())
		auth, err := exec.Command("bash", "-c", fmt.Sprintf("echo %s | base64 -d", password)).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to get authorization token")

		g.By("Mirror an image to ECR")
		tempDataDir, err := extractPullSecret(oc)
		defer os.RemoveAll(tempDataDir)
		o.Expect(err).NotTo(o.HaveOccurred())
		originAuth := filepath.Join(tempDataDir, ".dockerconfigjson")
		authFile, err := appendPullSecretAuth(originAuth, strings.Split(repositoryUri, "/")[0], "", string(auth))
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("image").Args("mirror", "quay.io/openshifttest/pause@sha256:e481caec2eb984ce023673a3ec280bf57dea8c0305009e246b019b3eef044f9e", repositoryUri+":latest", "--insecure", "-a", authFile, "--keep-manifest-list=true").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to mirror image to ECR")

		g.By("Create a new app using the image on ECR")
		err = oc.AsAdmin().WithoutNamespace().Run("new-app").Args("--name=hello-ecr", "--image="+repositoryUri+":latest", "--allow-missing-images", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create a new app with the image on ECR")

		g.By("Wait the pod to be running")
		ecrPodLabel := exutil.ParseLabelsOrDie("deployment=hello-ecr")
		_, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), ecrPodLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "ECR private image pod was not running after 4 minutes")
	})

	// OCP-72119 - Pull images from GCR repository should succeed
	// author: zhsun@redhat.com
	g.It("be able to pull images from GCR", func() {
		exutil.SkipIf(oc, "GCP")
		g.By("Create a new app using the image on GCR")
		err := oc.AsAdmin().WithoutNamespace().Run("new-app").Args("--name=hello-gcr", "--image=gcr.io/k8s-authenticated-test/agnhost:2.6", "--allow-missing-images", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create a new app with the image on GCR")

		g.By("Wait the pod to be running")
		gcrPodLabel := exutil.ParseLabelsOrDie("deployment=hello-gcr")
		_, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), gcrPodLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "GCR private image pod was not running after 4 minutes")
	})

	// OCP-72120 - Pull images from ACR repository should succeed
	// author: zhsun@redhat.com
	g.It("be able to pull images from ACR", func() {
		exutil.SkipIf(oc, "Azure")
		azureCloudName, azureErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.azure.cloudName}").Output()
		o.Expect(azureErr).NotTo(o.HaveOccurred())
		if azureCloudName == "AzureStackCloud" || azureCloudName == "AzureUSGovernmentCloud" {
			g.Skip("Skip for ASH and azure Gov due to we didn't create container registry on them!")
		}

		g.By("Create a container repository and get authorization token")
		registryName := "acrregistry" + getRandomString()
		resourceGroup, err := exutil.GetAzureCredentialFromCluster(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to get authorization token")
		az, sessErr := exutil.NewAzureSessionFromEnv()
		o.Expect(sessErr).NotTo(o.HaveOccurred())
		region := exutil.GetClusterRegion(oc)
		err = exutil.CreateAzureContainerRegistry(az, registryName, resourceGroup, region)
		defer exutil.DeleteAzureContainerRegistry(az, registryName, resourceGroup)
		user, password, _ := exutil.GetAzureContainerRepositoryCredential(az, registryName, resourceGroup)

		g.By("Mirror an image to ACR")
		tempDataDir, err := extractPullSecret(oc)
		defer os.RemoveAll(tempDataDir)
		o.Expect(err).NotTo(o.HaveOccurred())
		originAuth := filepath.Join(tempDataDir, ".dockerconfigjson")
		authFile, err := appendPullSecretAuth(originAuth, registryName+".azurecr.io", user, password)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("image").Args("mirror", "quay.io/openshifttest/pause@sha256:e481caec2eb984ce023673a3ec280bf57dea8c0305009e246b019b3eef044f9e", registryName+".azurecr.io/hello-acr:latest", "--insecure", "-a", authFile, "--keep-manifest-list=true").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to mirror image to ACR")

		g.By("Create a new app using the image on ACR")
		err = oc.AsAdmin().WithoutNamespace().Run("new-app").Args("--name=hello-acr", "--image="+registryName+".azurecr.io/hello-acr:latest", "--allow-missing-images", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create a new app with the image on ACR")

		g.By("Wait the pod to be running")
		acrPodLabel := exutil.ParseLabelsOrDie("deployment=hello-acr")
		_, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), acrPodLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "ACR private image pod was not running after 4 minutes")
	})
})

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func appendPullSecretAuth(authFile, regRouter, newRegUser, newRegPass string) (string, error) {
	fieldValue := ""
	if newRegUser == "" {
		fieldValue = newRegPass
	} else {
		fieldValue = newRegUser + ":" + newRegPass
	}
	regToken := base64.StdEncoding.EncodeToString([]byte(fieldValue))
	authDir, _ := filepath.Split(authFile)
	newAuthFile := filepath.Join(authDir, fmt.Sprintf("%s.json", getRandomString()))
	jqCMD := fmt.Sprintf(`cat %s | jq '.auths += {"%s":{"auth":"%s"}}' > %s`, authFile, regRouter, regToken, newAuthFile)
	_, err := exec.Command("bash", "-c", jqCMD).Output()
	if err != nil {
		e2e.Logf("Fail to extract dockerconfig: %v", err)
		return newAuthFile, err
	}
	return newAuthFile, nil
}

func extractPullSecret(oc *exutil.CLI) (string, error) {
	tempDataDir := filepath.Join("/tmp/", fmt.Sprintf("registry-%s", getRandomString()))
	err := os.Mkdir(tempDataDir, 0o755)
	if err != nil {
		e2e.Logf("Fail to create directory: %v", err)
		return tempDataDir, err
	}
	err = oc.AsAdmin().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", "--confirm", "--to="+tempDataDir).Execute()
	if err != nil {
		e2e.Logf("Fail to extract dockerconfig: %v", err)
		return tempDataDir, err
	}
	return tempDataDir, nil
}
