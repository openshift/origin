package generate

import (
	"fmt"
	"io"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/api/latest"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	dh "github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/dockerregistry"
	genapp "github.com/openshift/origin/pkg/generate/app"
	gen "github.com/openshift/origin/pkg/generate/generator"
	"github.com/openshift/origin/pkg/generate/source"
)

const longDescription = `
Experimental command

Generate configuration to build and deploy code in OpenShift from a source code
repository.

Docker builds - If a Dockerfile is present in the source code repository, then
a docker build is generated.

STI builds - If no builder image is specified as an argument, generate will detect
the type of source repository (JEE, Ruby, NodeJS) and associate a default builder
to it.

Services and Exposed Ports - For Docker builds, generate looks for EXPOSE directives
in the Dockerfile to determine which ports to expose. For STI builds, generate will
use the exposed ports of the builder image. In either case, if a different set of
ports needs to be exposed, use the --ports flag to specify them. Services will be
generated using these ports as well.


Usage:
openshift ex generate [source]

The source parameter may be a directory or a repository URL.
If not specified, the current directory is used.

Examples:

    $ openshift ex generate
    <finds a git repository in the current directory and builds artifacts based on detection>

    $ openshift ex generate ./repo/dir
    <specify the directory for the repository to use>

    $ openshift ex generate https://github.com/openshift/ruby-hello-world.git
    <use a remote git repository>

    $ openshift ex generate --builder-image=openshift/ruby-20-centos
    <force the application to use the specific builder-image>`

type params struct {
	name,
	sourceDir,
	sourceRef,
	sourceURL,
	dockerContext,
	builderImage,
	ports string
	env cmdutil.Environment
}

func NewCmdGenerate(name string) *cobra.Command {
	cfg := clientcmd.NewConfig()
	dockerHelper := dh.NewHelper()
	input := params{}
	var factory *cmd.Factory

	c := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Generates an application configuration from a source repository",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			_, osClient, err := cfg.Clients()
			if err != nil {
				osClient = nil
			}
			dockerClient, _, err := dockerHelper.GetClient()
			if err != nil {
				osClient = nil
			}
			if len(args) == 1 {
				if source.IsRemoteRepository(args[0]) {
					input.sourceURL = args[0]
				} else {
					input.sourceDir = args[0]
				}
			}
			if len(input.sourceDir) == 0 && len(input.sourceURL) == 0 {
				if input.sourceDir, err = os.Getwd(); err != nil {
					exitWithError(err)
				}
			}
			if envParam := kubecmd.GetFlagString(c, "environment"); len(envParam) > 0 {
				envVars := strings.Split(envParam, ",")
				env, _, errs := cmdutil.ParseEnvironmentArguments(envVars)
				if len(errs) > 0 {
					exitWithError(errors.NewAggregate(errs))
				}
				input.env = env
			}
			namespace, err := factory.DefaultNamespace(c)
			if err != nil {
				namespace = ""
			}
			imageResolver := newImageResolver(namespace, osClient, dockerClient)

			if err = generateApp(input, imageResolver, os.Stdout); err != nil {
				exitWithError(err)
			}
		},
	}
	clientConfig := cli.DefaultClientConfig(c.PersistentFlags())
	factory = cmd.NewFactory(clientConfig)
	factory.BindFlags(c.PersistentFlags())

	flag := c.Flags()
	flag.StringVar(&input.name, "name", "", "Set name to use for generated application artifacts")
	flag.StringVar(&input.sourceRef, "ref", "", "Set the name of the repository branch/ref to use")
	flag.StringVar(&input.sourceURL, "source-url", "", "Set the source URL")
	flag.StringVar(&input.dockerContext, "docker-context", "", "Context path for Dockerfile if creating a Docker build")
	flag.StringVar(&input.builderImage, "builder-image", "", "Image to use for STI build")
	flag.StringVarP(&input.ports, "ports", "p", "", "Comma-separated list of ports to expose on pod deployment")
	flag.StringP("environment", "e", "", "Comma-separated list of environment variables to add to the deployment. Should be in the form of var1=value1,var2=value2,...")
	dockerHelper.InstallFlags(flag)
	return c
}

func newImageResolver(namespace string, osClient osclient.Interface, dockerClient *docker.Client) genapp.Resolver {
	resolver := genapp.PerfectMatchWeightedResolver{}

	if dockerClient != nil {
		localDockerResolver := &genapp.DockerClientResolver{Client: dockerClient}
		resolver = append(resolver, genapp.WeightedResolver{localDockerResolver, 0.0})
	}

	if osClient != nil {
		namespaces := []string{}
		if len(namespace) > 0 {
			namespaces = append(namespaces, namespace)
		}
		namespaces = append(namespaces, "default")
		imageStreamResolver := &genapp.ImageStreamResolver{
			Client:     osClient,
			Images:     osClient,
			Namespaces: namespaces,
		}
		resolver = append(resolver, genapp.WeightedResolver{imageStreamResolver, 0.0})
	}

	dockerRegistryResolver := &genapp.DockerRegistryResolver{dockerregistry.NewClient()}
	resolver = append(resolver, genapp.WeightedResolver{dockerRegistryResolver, 0.0})

	return resolver
}

func generateSourceRef(url string, dir string, ref string, name string) (*genapp.SourceRef, error) {
	srcRefGen := gen.NewSourceRefGenerator()
	var result *genapp.SourceRef
	var err error
	if len(url) > 0 {
		glog.V(3).Infof("Generating source reference from URL: %s", url)
		if result, err = srcRefGen.FromGitURL(url); err != nil {
			return nil, err
		}
	} else {
		glog.V(3).Infof("Generating source reference from directory: %s", dir)
		if result, err = srcRefGen.FromDirectory(dir); err != nil {
			return nil, err
		}
	}
	if len(ref) > 0 {
		result.Ref = ref
	}
	if len(name) > 0 {
		result.Name = name
	}
	return result, nil
}

func generateBuildStrategyRef(srcRef *genapp.SourceRef, dockerContext string, builderImage string) (*genapp.BuildStrategyRef, error) {
	strategyRefGen := gen.NewBuildStrategyRefGenerator(source.DefaultDetectors)
	imageRefGen := gen.NewImageRefGenerator()
	if len(dockerContext) > 0 {
		glog.V(3).Infof("Generating build strategy reference using dockerContext: %s", dockerContext)
		return strategyRefGen.FromSourceRefAndDockerContext(*srcRef, dockerContext)
	} else if len(builderImage) > 0 {
		glog.V(3).Infof("Generating build strategy reference using builder image: %s", builderImage)
		builderRef, err := imageRefGen.FromName(builderImage)
		if err != nil {
			return nil, err
		}
		return strategyRefGen.FromSTIBuilderImage(builderRef)
	} else {
		glog.V(3).Infof("Detecting build strategy using source reference: %#v", srcRef)
		return strategyRefGen.FromSourceRef(*srcRef)
	}
}

func generateOutputImageRef(srcRef *genapp.SourceRef, strategyRef *genapp.BuildStrategyRef, ports string) (*genapp.ImageRef, error) {
	imageRefGen := gen.NewImageRefGenerator()
	name, ok := srcRef.SuggestName()
	if !ok {
		return nil, fmt.Errorf("cannot suggest a name for the output image. Please specify one with the --output-image argument")
	}
	if len(ports) > 0 {
		portList := strings.Split(ports, ",")
		return imageRefGen.FromNameAndPorts(name, portList)
	}
	if strategyRef.IsDockerBuild {
		return imageRefGen.FromDockerfile(name, srcRef.Dir, strategyRef.DockerContext)
	} else {
		return imageRefGen.FromName(name)
	}
}

func generateApp(input params, imageResolver genapp.Resolver, out io.Writer) error {
	// Get a SourceRef
	srcRef, err := generateSourceRef(input.sourceURL, input.sourceDir, input.sourceRef, input.name)
	if err != nil {
		return err
	}
	glog.V(2).Infof("Source reference: %#v", srcRef)

	// Get a BuildStrategyRef
	strategyRef, err := generateBuildStrategyRef(srcRef, input.dockerContext, input.builderImage)
	if err != nil {
		return err
	}
	glog.V(2).Infof("Generated build strategy reference: %#v", strategyRef)

	// Get an ImageRef for Output
	outputRef, err := generateOutputImageRef(srcRef, strategyRef, input.ports)
	if err != nil {
		return err
	}
	glog.V(2).Infof("Generated output image reference: %#v", outputRef)

	outputRefCopy := *outputRef
	baseRef := &outputRefCopy
	if !strategyRef.IsDockerBuild {
		if baseImageMatch, err := imageResolver.Resolve(strategyRef.Base.RepoName()); err == nil {
			if inputRef, err := genapp.InputImageFromMatch(baseImageMatch); err == nil {
				if len(input.ports) > 0 {
					inputRef.Info = baseRef.Info
				}
				baseRef = inputRef
			}
		}
	}
	baseRef.AsImageRepository = false

	pipeline, err := genapp.NewBuildPipeline(outputRef.Name, baseRef, strategyRef, srcRef)
	if err != nil {
		return err
	}
	env := genapp.Environment{}
	for k, v := range input.env {
		env[k] = v
	}
	if err := pipeline.NeedsDeployment(env); err != nil {
		return err
	}

	objects, err := pipeline.Objects(genapp.NewAcceptFirst())
	if err != nil {
		return err
	}
	objects = genapp.AddServices(objects)
	list := &kapi.List{Items: objects}
	output, err := latest.Codec.Encode(list)
	if err != nil {
		return err
	}
	_, err = out.Write(output)
	return err
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
