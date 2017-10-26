/*
Copyright 2017 The Kubernetes Authors.

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

package pretty

import (
	"fmt"
)

// ContextBuilder allows building up pretty message lines with context
// that is important for debugging and tracing. This class helps create log
// line formatting consistency. Pretty lines should be in the form:
// <Kind> "<Namespace>/<Name>": <message>
type ContextBuilder struct {
	Kind      Kind
	Namespace string
	Name      string
}

// NewContextBuilder returns a new ContextBuilder that can be used to format messages in the
// form `<Kind> "<Namespace>/<Name>": <message>`.
// kind,  namespace, name are all optional.
func NewContextBuilder(kind Kind, namespace string, name string) *ContextBuilder {
	lb := new(ContextBuilder)
	lb.Kind = kind
	lb.Namespace = namespace
	lb.Name = name
	return lb
}

// SetKind sets the kind to use in the source context for messages.
func (pcb *ContextBuilder) SetKind(k Kind) *ContextBuilder {
	pcb.Kind = k
	return pcb
}

// SetNamespace sets the namespace to use in the source context for messages.
func (pcb *ContextBuilder) SetNamespace(n string) *ContextBuilder {
	pcb.Namespace = n
	return pcb
}

// SetName sets the name to use in the source context for messages.
func (pcb *ContextBuilder) SetName(n string) *ContextBuilder {
	pcb.Name = n
	return pcb
}

// Message returns a string with message prepended with the current source context.
func (pcb *ContextBuilder) Message(msg string) string {
	if pcb.Kind > 0 || pcb.Namespace != "" || pcb.Name != "" {
		return fmt.Sprintf(`%s: %s`, pcb, msg)
	}
	return msg
}

// Messagef returns a string with message formatted then prepended with the current source context.
func (pcb *ContextBuilder) Messagef(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return pcb.Message(msg)
}

// TODO(n3wscott): Support <type> (K8S: <K8S-Type-Name> ExternalName: <External-Type-Name>)

func (pcb ContextBuilder) String() string {
	s := ""
	space := ""
	if pcb.Kind > 0 {
		s += fmt.Sprintf("%s", pcb.Kind)
		space = " "
	}
	if pcb.Namespace != "" && pcb.Name != "" {
		s += fmt.Sprintf(`%s"%s/%s"`, space, pcb.Namespace, pcb.Name)
	} else if pcb.Namespace != "" {
		s += fmt.Sprintf(`%s"%s"`, space, pcb.Namespace)
	} else if pcb.Name != "" {
		s += fmt.Sprintf(`%s"%s"`, space, pcb.Name)
	}
	return s
}
