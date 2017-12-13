package set

import (
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

func selectContainers(containers []kapi.Container, spec string) ([]*kapi.Container, []*kapi.Container) {
	out := []*kapi.Container{}
	skipped := []*kapi.Container{}
	for i, c := range containers {
		if selectString(c.Name, spec) {
			out = append(out, &containers[i])
		} else {
			skipped = append(skipped, &containers[i])
		}
	}
	return out, skipped
}

func handlePodUpdateError(out io.Writer, err error, resource string) {
	if statusError, ok := err.(*errors.StatusError); ok && errors.IsInvalid(err) {
		errorDetails := statusError.Status().Details
		if errorDetails.Kind == "Pod" {
			all, match := true, false
			for _, cause := range errorDetails.Causes {
				if cause.Field == "spec" && strings.Contains(cause.Message, "may not update fields other than") {
					fmt.Fprintf(out, "error: may not update %s in pod %q directly\n", resource, errorDetails.Name)
					match = true
				} else {
					all = false
				}
			}
			if all && match {
				return
			}
		} else {
			if ok := kcmdutil.PrintErrorWithCauses(err, out); ok {
				return
			}
		}
	}

	fmt.Fprintf(out, "error: %v\n", err)
}

// selectString returns true if the provided string matches spec, where spec is a string with
// a non-greedy '*' wildcard operator.
// TODO: turn into a regex and handle greedy matches and backtracking.
func selectString(s, spec string) bool {
	if spec == "*" {
		return true
	}
	if !strings.Contains(spec, "*") {
		return s == spec
	}

	pos := 0
	match := true
	parts := strings.Split(spec, "*")
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		next := strings.Index(s[pos:], part)
		switch {
		// next part not in string
		case next < pos:
			fallthrough
		// first part does not match start of string
		case i == 0 && pos != 0:
			fallthrough
		// last part does not exactly match remaining part of string
		case i == (len(parts)-1) && len(s) != (len(part)+next):
			match = false
			break
		default:
			pos = next
		}
	}
	return match
}

func updateEnv(existing []kapi.EnvVar, env []kapi.EnvVar, remove []string) []kapi.EnvVar {
	out := []kapi.EnvVar{}
	covered := sets.NewString(remove...)
	for _, e := range existing {
		if covered.Has(e.Name) {
			continue
		}
		newer, ok := findEnv(env, e.Name)
		if ok {
			covered.Insert(e.Name)
			out = append(out, newer)
			continue
		}
		out = append(out, e)
	}
	for _, e := range env {
		if covered.Has(e.Name) {
			continue
		}
		covered.Insert(e.Name)
		out = append(out, e)
	}
	return out
}

func findEnv(env []kapi.EnvVar, name string) (kapi.EnvVar, bool) {
	for _, e := range env {
		if e.Name == name {
			return e, true
		}
	}
	return kapi.EnvVar{}, false
}

// Patch represents the result of a mutation to an object.
type Patch struct {
	Info *resource.Info
	Err  error

	Before []byte
	After  []byte
	Patch  []byte
}

// CalculatePatches calls the mutation function on each provided info object, and generates a strategic merge patch for
// the changes in the object. Encoder must be able to encode the info into the appropriate destination type. If mutateFn
// returns false, the object is not included in the final list of patches.
func CalculatePatches(infos []*resource.Info, encoder runtime.Encoder, mutateFn func(*resource.Info) (bool, error)) []*Patch {
	var patches []*Patch
	for _, info := range infos {
		patch := &Patch{Info: info}

		versionedEncoder := legacyscheme.Codecs.EncoderForVersion(encoder, patch.Info.Mapping.GroupVersionKind.GroupVersion())
		patch.Before, patch.Err = runtime.Encode(versionedEncoder, info.Object)

		ok, err := mutateFn(info)
		if !ok {
			continue
		}
		if err != nil {
			patch.Err = err
		}
		patches = append(patches, patch)
		if patch.Err != nil {
			continue
		}

		patch.After, patch.Err = runtime.Encode(versionedEncoder, info.Object)
		if patch.Err != nil {
			continue
		}

		// TODO: should be via New
		versioned, err := info.Mapping.ConvertToVersion(info.Object, info.Mapping.GroupVersionKind.GroupVersion())
		if err != nil {
			patch.Err = err
			continue
		}

		patch.Patch, patch.Err = strategicpatch.CreateTwoWayMergePatch(patch.Before, patch.After, versioned)
	}
	return patches
}
