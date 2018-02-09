/*
Copyright 2018 The Kubernetes Authors.

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

package output

import (
	"fmt"
	"strings"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	statusActive     = "Active"
	statusDeprecated = "Deprecated"
)

func formatStatusShort(condition string, conditionStatus v1beta1.ConditionStatus, reason string) string {
	if conditionStatus == v1beta1.ConditionTrue {
		return condition
	}
	return reason
}

func formatStatusFull(condition string, conditionStatus v1beta1.ConditionStatus, reason string, message string, timestamp v1.Time) string {
	status := formatStatusShort(condition, conditionStatus, reason)
	if status == "" {
		return ""
	}

	message = strings.TrimRight(message, ".")
	return fmt.Sprintf("%s - %s @ %s", status, message, timestamp.UTC())
}
