package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/generate/app"
	imageapi "github.com/openshift/origin/pkg/image/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

func localOrRemoteName(meta kapi.ObjectMeta, namespace string) string {
	if len(meta.Namespace) == 0 || namespace == meta.Namespace {
		return meta.Name
	}
	return fmt.Sprintf("%s in project %s", meta.Name, meta.Namespace)
}

func extractFirstImageStreamTag(newOnly bool, images ...*app.ImageRef) string {
	for _, image := range images {
		if image == nil {
			continue
		}
		if image.Exists() && newOnly {
			break
		}
		ref := image.ObjectReference()
		// if the reference is to an IST, the image is intended to be an IST
		if ref.Kind != "ImageStreamTag" || !image.AsImageStream {
			break
		}
		return ref.Name
	}
	return ""
}

func describeLocatedImage(refInput *app.ComponentInput, baseNamespace string) string {
	match := refInput.ResolvedMatch
	switch {
	case match.ImageStream != nil:
		if image := match.Image; image != nil {
			shortID := imageapi.ShortDockerImageID(image, 7)
			if !image.Created.IsZero() {
				shortID = fmt.Sprintf("%s (%s old)", shortID, describe.FormatRelativeTime(image.Created.Time))
			}
			return fmt.Sprintf("Found image %s in image stream %q under tag :%s for %q", shortID, localOrRemoteName(match.ImageStream.ObjectMeta, baseNamespace), match.ImageTag, refInput)
		}
		return fmt.Sprintf("Found tag :%s in image stream %q for %q", match.ImageTag, localOrRemoteName(match.ImageStream.ObjectMeta, baseNamespace), refInput)
	case match.Image != nil:
		image := match.Image
		shortID := imageapi.ShortDockerImageID(image, 7)
		if !image.Created.IsZero() {
			shortID = fmt.Sprintf("%s (%s old)", shortID, describe.FormatRelativeTime(image.Created.Time))
		}
		return fmt.Sprintf("Found Docker image %s from %s for %q", shortID, match.Meta["registry"], refInput)
	default:
		return ""
	}
}

func describeBuildPipelineWithImage(out io.Writer, ref app.ComponentReference, pipeline *app.Pipeline, baseNamespace string) {
	refInput := ref.Input()
	match := refInput.ResolvedMatch

	if locatedImage := describeLocatedImage(refInput, baseNamespace); len(locatedImage) > 0 {
		fmt.Fprintf(out, "--> %s\n", locatedImage)
	}

	trackedImage := extractFirstImageStreamTag(true, pipeline.InputImage, pipeline.Image)
	if len(trackedImage) > 0 {
		fmt.Fprintf(out, "    * An image stream will be created as %q that will track this image\n", trackedImage)
	}
	if pipeline.Build != nil {
		if refInput.Uses != nil && refInput.Uses.Info() != nil {
			matches := []string{}
			for _, t := range refInput.Uses.Info().Types {
				if len(t.Platform) == 0 {
					continue
				}
				if len(t.Version) > 0 {
					matches = append(matches, fmt.Sprintf("%s %s", t.Platform, t.Version))
				}
				matches = append(matches, t.Platform)
			}
			if len(matches) > 0 {
				fmt.Fprintf(out, "    * The source repository appears to match: %s\n", strings.Join(matches, ", "))
			}
		}
		var strategy string
		if pipeline.Build.Strategy.IsDockerBuild {
			strategy = "Docker"
		} else {
			strategy = "source"
		}
		var source string
		switch s := pipeline.Build.Source; {
		case s.Binary:
			source = "binary input"
		case len(s.DockerfileContents) > 0:
			source = "a predefined Dockerfile"
		case s.URL != nil:
			source = fmt.Sprintf("source code from %s", s.URL)
		default:
			source = "<unknown>"
		}

		fmt.Fprintf(out, "    * A %s build using %s will be created\n", strategy, source)
		if buildOut, err := pipeline.Build.Output.BuildOutput(); err == nil && buildOut != nil && buildOut.To != nil {
			switch to := buildOut.To; {
			case to.Kind == "ImageStreamTag":
				fmt.Fprintf(out, "      * The resulting image will be pushed to image stream %q\n", to.Name)
			case to.Kind == "DockerImage":
				fmt.Fprintf(out, "      * The resulting image will be pushed with Docker to %q\n", to.Name)
			default:
				fmt.Fprintf(out, "      * The resulting image will be pushed to %s %q\n", to.Kind, to.Name)
			}
		}
		if len(trackedImage) > 0 && !pipeline.Build.Source.Binary {
			fmt.Fprintf(out, "      * Every time %q changes a new build will be triggered\n", trackedImage)
		}
	}
	if pipeline.Deployment != nil {
		if len(pipeline.Deployment.Images) > 1 {
			fmt.Fprintf(out, "    * This image will be deployed as part of deployment config %q\n", pipeline.Deployment.Name)
		} else {
			fmt.Fprintf(out, "    * This image will be deployed in deployment config %q\n", pipeline.Deployment.Name)
		}

		if pipeline.Image != nil && pipeline.Image.HasEmptyDir {
			fmt.Fprintf(out, "    * This image declares volumes and will default to use non-persistent, host-local storage.\n")
			fmt.Fprintf(out, "      You can add persistent volumes later by running 'volume dc/%s --add ...'\n", pipeline.Deployment.Name)
		}
	}
	if match.Image != nil {
		if pipeline.Deployment != nil {
			ports := sets.NewString()
			if match.Image.Config != nil {
				for k := range match.Image.Config.ExposedPorts {
					ports.Insert(k)
				}
			}
			switch len(ports) {
			case 0:
				fmt.Fprintf(out, "    * The image does not expose any ports - if you want to load balance or send traffic to this component\n")
				fmt.Fprintf(out, "      you will need to create a service with 'expose dc/%s --port=[port]' later\n", pipeline.Deployment.Name)
			default:
				orderedPorts := ports.List()
				sort.Sort(sort.StringSlice(orderedPorts))
				if len(orderedPorts) == 1 {
					fmt.Fprintf(out, "    * Port %s will be load balanced by service %q\n", orderedPorts[0], pipeline.Deployment.Name)
				} else {
					fmt.Fprintf(out, "    * Ports %s will be load balanced by service %q\n", strings.Join(orderedPorts, ", "), pipeline.Deployment.Name)
				}
			}
		}
	}
}

func describeGeneratedTemplate(out io.Writer, ref app.ComponentReference, result *templateapi.Template, baseNamespace string) {
	fmt.Fprintf(out, "--> Deploying template %s for %q\n", localOrRemoteName(ref.Input().ResolvedMatch.Template.ObjectMeta, baseNamespace), ref.Input())
	if len(result.Parameters) > 0 {
		fmt.Fprintf(out, "     With parameters:\n")
		for _, p := range result.Parameters {
			name := p.DisplayName
			if len(name) == 0 {
				name = p.Name
			}
			var generated string
			if len(p.Generate) > 0 {
				generated = " # generated"
			}
			fmt.Fprintf(out, "      %s=%s%s\n", name, p.Value, generated)
		}
	}
}

func describeGeneratedJob(out io.Writer, ref app.ComponentReference, pod *kapi.Pod, secret *kapi.Secret, baseNamespace string) {
	refInput := ref.Input()
	generatorInput := refInput.ResolvedMatch.GeneratorInput
	hasToken := generatorInput.Token != nil

	fmt.Fprintf(out, "--> Installing application from %q\n", refInput)
	if locatedImage := describeLocatedImage(refInput, baseNamespace); len(locatedImage) > 0 {
		fmt.Fprintf(out, "    * %s\n", locatedImage)
	}

	fmt.Fprintf(out, "    * Install will run in pod %q\n", localOrRemoteName(pod.ObjectMeta, baseNamespace))
	switch {
	case secret != nil:
		fmt.Fprintf(out, "    * The pod has access to your current session token through the secret %q\n", localOrRemoteName(secret.ObjectMeta, baseNamespace))
		fmt.Fprintf(out, "    * If you cancel the install, you should delete the secret or log out of your session\n")
	case hasToken && generatorInput.Token.Env != nil:
		fmt.Fprintf(out, "    * The pod has access to your current session token via environment variable %s\n", *generatorInput.Token.Env)
		fmt.Fprintf(out, "    * If you cancel the install, you should delete the pod or log out of your session\n")
	case hasToken:
		fmt.Fprintf(out, "    * The pod has access to your current session token. Please delete the pod if you cancel the install\n")
	}
	if hasToken {
		fmt.Fprintf(out, "--> WARNING: The pod requires access to your current session token to install this image. Only\n")
		fmt.Fprintf(out, "      grant access to images whose source you trust. The image will be able to perform any\n")
		fmt.Fprintf(out, "      action you can take on the cluster.\n")
	}
}
