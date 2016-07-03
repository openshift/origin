/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubectl

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	deploymentutil "k8s.io/kubernetes/pkg/util/deployment"
	"k8s.io/kubernetes/pkg/util/errors"
)

const (
	ChangeCauseAnnotation = "kubernetes.io/change-cause"
)

// HistoryViewer provides an interface for resources that have historical information.
type HistoryViewer interface {
	ViewHistory(namespace, name string, revision int64) (string, error)
}

func HistoryViewerFor(kind unversioned.GroupKind, c clientset.Interface) (HistoryViewer, error) {
	switch kind {
	case extensions.Kind("Deployment"):
		return &DeploymentHistoryViewer{c}, nil
	}
	return nil, fmt.Errorf("no history viewer has been implemented for %q", kind)
}

type DeploymentHistoryViewer struct {
	c clientset.Interface
}

// ViewHistory prints the revision history of a deployment
func (h *DeploymentHistoryViewer) ViewHistory(namespace, name string, revision int64) (string, error) {
	deployment, err := h.c.Extensions().Deployments(namespace).Get(name)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve deployment %s: %v", name, err)
	}
	_, allOldRSs, err := deploymentutil.GetOldReplicaSets(deployment, h.c)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve old replica sets from deployment %s: %v", name, err)
	}
	newRS, err := deploymentutil.GetNewReplicaSet(deployment, h.c)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve new replica set from deployment %s: %v", name, err)
	}

	historyInfo := make(map[int64]*api.PodTemplateSpec)
	for _, rs := range append(allOldRSs, newRS) {
		v, err := deploymentutil.Revision(rs)
		if err != nil {
			continue
		}
		historyInfo[v] = &rs.Spec.Template
		changeCause := getChangeCause(rs)
		if historyInfo[v].Annotations == nil {
			historyInfo[v].Annotations = make(map[string]string)
		}
		if len(changeCause) > 0 {
			historyInfo[v].Annotations[ChangeCauseAnnotation] = changeCause
		}
	}

	if len(historyInfo) == 0 {
		return "No rollout history found.", nil
	}

	if revision > 0 {
		// Print details of a specific revision
		template, ok := historyInfo[revision]
		if !ok {
			return "", fmt.Errorf("unable to find the specified revision")
		}
		buf := bytes.NewBuffer([]byte{})
		DescribePodTemplate(template, buf)
		return buf.String(), nil
	}

	// Sort the revisionToChangeCause map by revision
	var revisions []string
	for k := range historyInfo {
		revisions = append(revisions, strconv.FormatInt(k, 10))
	}
	sort.Strings(revisions)

	return tabbedString(func(out io.Writer) error {
		fmt.Fprintf(out, "REVISION\tCHANGE-CAUSE\n")
		errs := []error{}
		for _, r := range revisions {
			// Find the change-cause of revision r
			r64, err := strconv.ParseInt(r, 10, 64)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			changeCause := historyInfo[r64].Annotations[ChangeCauseAnnotation]
			if len(changeCause) == 0 {
				changeCause = "<none>"
			}
			fmt.Fprintf(out, "%s\t%s\n", r, changeCause)
		}
		return errors.NewAggregate(errs)
	})
}

// getChangeCause returns the change-cause annotation of the input object
func getChangeCause(obj runtime.Object) string {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return ""
	}
	return accessor.GetAnnotations()[ChangeCauseAnnotation]
}
