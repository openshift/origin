package config

import (
	"fmt"
	"reflect"

	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// MergeConfig takes a haystack to look for existing stanzas in (probably the merged config), a config object to modify (probably
// either the local or envvar config), and the new additions to merge in.  It tries to find equivalents for the addition inside of the
// haystack and uses the mapping to avoid creating additional stanzas with duplicate information.  It either locates or original
// stanzas or creates new ones for clusters and users.  Then it uses the mapped names to build the correct contexts
func MergeConfig(haystack, toModify, addition clientcmdapi.Config) (*clientcmdapi.Config, error) {
	ret := toModify

	requestedClusterNamesToActualClusterNames := map[string]string{}
	existingClusterNames, err := getMapKeys(reflect.ValueOf(haystack.Clusters))
	if err != nil {
		return nil, err
	}
	for requestedKey, needle := range addition.Clusters {
		if existingName := FindExistingClusterName(haystack, needle); len(existingName) > 0 {
			requestedClusterNamesToActualClusterNames[requestedKey] = existingName
			continue
		}

		uniqueName := getUniqueName(requestedKey, existingClusterNames)
		requestedClusterNamesToActualClusterNames[requestedKey] = uniqueName
		ret.Clusters[uniqueName] = needle
	}

	requestedAuthInfoNamesToActualAuthInfoNames := map[string]string{}
	existingAuthInfoNames, err := getMapKeys(reflect.ValueOf(haystack.AuthInfos))
	if err != nil {
		return nil, err
	}
	for requestedKey, needle := range addition.AuthInfos {
		if existingName := FindExistingAuthInfoName(haystack, needle); len(existingName) > 0 {
			requestedAuthInfoNamesToActualAuthInfoNames[requestedKey] = existingName
			continue
		}

		uniqueName := getUniqueName(requestedKey, existingAuthInfoNames)
		requestedAuthInfoNamesToActualAuthInfoNames[requestedKey] = uniqueName
		ret.AuthInfos[uniqueName] = needle
	}

	requestedContextNamesToActualContextNames := map[string]string{}
	existingContextNames, err := getMapKeys(reflect.ValueOf(haystack.Contexts))
	if err != nil {
		return nil, err
	}
	for requestedKey, needle := range addition.Contexts {
		exists := false

		actualContext := clientcmdapi.NewContext()
		actualContext.AuthInfo, exists = requestedAuthInfoNamesToActualAuthInfoNames[needle.AuthInfo]
		if !exists {
			actualContext.AuthInfo = needle.AuthInfo
		}
		actualContext.Cluster, exists = requestedClusterNamesToActualClusterNames[needle.Cluster]
		if !exists {
			actualContext.Cluster = needle.Cluster
		}
		actualContext.Namespace = needle.Namespace
		actualContext.Extensions = needle.Extensions

		if existingName := FindExistingContextName(haystack, *actualContext); len(existingName) > 0 {
			// if this already exists, just move to the next, our job is done
			requestedContextNamesToActualContextNames[requestedKey] = existingName
			continue
		}

		uniqueName := getUniqueName(actualContext.Cluster+"-"+actualContext.AuthInfo, existingContextNames)
		requestedContextNamesToActualContextNames[requestedKey] = uniqueName
		ret.Contexts[uniqueName] = *actualContext
	}

	if len(addition.CurrentContext) > 0 {
		if newCurrentContext, exists := requestedContextNamesToActualContextNames[addition.CurrentContext]; exists {
			ret.CurrentContext = newCurrentContext
		} else {
			ret.CurrentContext = addition.CurrentContext
		}
	}

	return &ret, nil
}

// FindExistingClusterName finds the nickname for the passed cluster config
func FindExistingClusterName(haystack clientcmdapi.Config, needle clientcmdapi.Cluster) string {
	for key, cluster := range haystack.Clusters {
		if reflect.DeepEqual(cluster, needle) {
			return key
		}
	}

	return ""
}

// FindExistingAuthInfoName finds the nickname for the passed auth info
func FindExistingAuthInfoName(haystack clientcmdapi.Config, needle clientcmdapi.AuthInfo) string {
	for key, authInfo := range haystack.AuthInfos {
		if reflect.DeepEqual(authInfo, needle) {
			return key
		}
	}

	return ""
}

// FindExistingContextName finds the nickname for the passed context
func FindExistingContextName(haystack clientcmdapi.Config, needle clientcmdapi.Context) string {
	for key, context := range haystack.Contexts {
		if reflect.DeepEqual(context, needle) {
			return key
		}
	}

	return ""
}

func getMapKeys(theMap reflect.Value) (*util.StringSet, error) {
	if theMap.Kind() != reflect.Map {
		return nil, fmt.Errorf("theMap must be of type %v, not %v", reflect.Map, theMap.Kind())
	}

	ret := &util.StringSet{}

	switch theMap.Kind() {
	case reflect.Map:
		for _, keyValue := range theMap.MapKeys() {
			ret.Insert(keyValue.String())
		}

	}

	return ret, nil

}

func getUniqueName(basename string, existingNames *util.StringSet) string {
	if !existingNames.Has(basename) {
		return basename
	}

	for i := 0; i < 100; i++ {
		trialName := fmt.Sprintf("%v-%d", basename, i)
		if !existingNames.Has(trialName) {
			return trialName
		}
	}

	return string(util.NewUUID())
}
