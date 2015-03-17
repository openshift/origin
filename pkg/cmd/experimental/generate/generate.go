package generate

import (
	"fmt"
	"io"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	osclient "github.com/openshift/origin/pkg/client"
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

Services and Exposed Port - For Docker builds, generate looks for EXPOSE directives
in the Dockerfile to determine which port to expose. For STI builds, generate will
use the exposed port of the builder image. In either case, if a different port
needs to be exposed, use the --port flag to specify them. Services will be
generated using this port as well.

The source parameter may be a directory or a repository URL.
If not specified, the current directory is used.

Examples:

    # Find a git repository in the current directory and build artifacts based on detection
    $ openshift ex generate

    # Specify the directory for the repository to use
    $ openshift ex generate ./repo/dir

    # Use a remote git repository
    $ openshift ex generate https://github.com/openshift/ruby-hello-world.git

    # Force the application to use the specific builder-image
    $ openshift ex generate --builder-image=openshift/ruby-20-centos7
`

type params struct {
	name,
	sourceDir,
	sourceRef,
	sourceURL,
	dockerContext,
	builderImage,
	port string
	env genapp.Environment
}

// NewCmdGenerate creates a new generate command. The generate command will generate configuration
// based on a source repository
func NewCmdGenerate(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	dockerHelper := dh.NewHelper()
	input := params{}
	environment := ""

	c := &cobra.Command{
		Use:   fmt.Sprintf("%s [source]", name),
		Short: "Generates an application configuration from a source repository",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			var err error

			// Determine which source to use
			input.sourceDir, input.sourceURL, err = getSource(input.sourceURL, args, os.Getwd)
			checkErr(err)

			// Create an image resolver
			resolver := getResolver(getNamespace(f, c), getOSClient(f, c), getDockerClient(dockerHelper))
			checkErr(err)

			// Get environment variables
			input.env, err = getEnvironment(environment)
			checkErr(err)

			// Generate config
			generator := &appGenerator{
				input:          input,
				resolver:       resolver,
				srcRefGen:      gen.NewSourceRefGenerator(),
				strategyRefGen: gen.NewBuildStrategyRefGenerator(source.DefaultDetectors, resolver),
				imageRefGen:    gen.NewImageRefGenerator(),
			}
			list, err := generator.run()
			checkErr(err)

			// Output config
			setDefaultPrinter(c)
			err = f.Factory.PrintObject(c, list, out)
			checkErr(err)
		},
	}

	flag := c.Flags()
	flag.StringVar(&input.name, "name", "", "Set name to use for generated application artifacts")
	flag.StringVar(&input.sourceRef, "ref", "", "Set the name of the repository branch/ref to use")
	flag.StringVar(&input.sourceURL, "source-url", "", "Set the source URL")
	flag.StringVar(&input.dockerContext, "docker-context", "", "Context path for Dockerfile if creating a Docker build")
	flag.StringVar(&input.builderImage, "builder-image", "", "Image to use for STI build")
	flag.StringVarP(&input.port, "port", "p", "", "Port to expose on pod deployment")
	flag.StringVarP(&environment, "environment", "e", "", "Comma-separated list of environment variables to add to the deployment."+
		"Should be in the form of var1=value1,var2=value2,...")
	kcmdutil.AddPrinterFlags(c)
	dockerHelper.InstallFlags(flag)
	return c
}

type getDirFunc func() (string, error)

func getSource(sourceURL string, args []string, getdir getDirFunc) (directory string, URL string, err error) {
	argument := ""
	if len(args) > 0 {
		argument = args[0]
	}
	if len(sourceURL) > 0 && len(argument) > 0 {
		err = fmt.Errorf("cannot specify both a sourceURL flag (--sourceURL=%s) and a source argument (%s)", sourceURL, argument)
		return
	}
	if len(sourceURL) > 0 {
		glog.V(3).Infof("Using source URL from --sourceURL flag: %s", sourceURL)
		URL = sourceURL
		return
	}
	if len(argument) > 0 {
		if genapp.IsRemoteRepository(argument) {
			glog.V(3).Infof("Using source URL argument: %s", argument)
			URL = argument
			return
		}
		glog.V(3).Infof("Using file system directory argument: %s", argument)
		directory = argument
		return
	}
	var getdirErr error
	directory, getdirErr = getdir()
	if getdirErr != nil {
		err = fmt.Errorf("cannot retrieve current directory: %v", getdirErr)
		return
	}
	glog.V(3).Infof("Using current directory for source: %s", directory)
	return
}

func setDefaultPrinter(c *cobra.Command) {
	flag := c.Flags().Lookup("output")
	if len(flag.Value.String()) == 0 {
		flag.Value.Set("json")
	}
}

func getOSClient(f *clientcmd.Factory, c *cobra.Command) osclient.Interface {
	osClient, _, err := f.Clients(c)
	if err != nil {
		glog.V(4).Infof("Error getting OpenShift client: %v", err)
		return nil
	}
	return osClient

}

func getNamespace(f *clientcmd.Factory, c *cobra.Command) string {
	ns, err := f.DefaultNamespace(c)
	if err != nil {
		glog.V(4).Infof("Error getting default namespace: %v", err)
		return ""
	}
	return ns
}

func getDockerClient(dh *dh.Helper) *docker.Client {
	dockerClient, _, err := dh.GetClient()
	if err != nil {
		glog.V(4).Infof("Error getting docker client: %v", err)
		return nil
	}
	return dockerClient
}

func getResolver(namespace string, osClient osclient.Interface, dockerClient *docker.Client) genapp.Resolver {
	resolver := genapp.PerfectMatchWeightedResolver{}

	if dockerClient != nil {
		localDockerResolver := &genapp.DockerClientResolver{Client: dockerClient}
		resolver = append(resolver, genapp.WeightedResolver{Resolver: localDockerResolver, Weight: 0.0})
	}

	if osClient != nil {
		namespaces := []string{}
		if len(namespace) > 0 {
			namespaces = append(namespaces, namespace)
		}
		namespaces = append(namespaces, "default")
		imageStreamResolver := &genapp.ImageStreamResolver{
			Client:            osClient,
			ImageStreamImages: osClient,
			Namespaces:        namespaces,
		}
		resolver = append(resolver, genapp.WeightedResolver{Resolver: imageStreamResolver, Weight: 0.0})
	}

	dockerRegistryResolver := &genapp.DockerRegistryResolver{dockerregistry.NewClient()}
	resolver = append(resolver, genapp.WeightedResolver{Resolver: dockerRegistryResolver, Weight: 0.0})

	return resolver
}

func getEnvironment(envParam string) (genapp.Environment, error) {
	if len(envParam) > 0 {
		envVars := strings.Split(envParam, ",")
		env, _, errs := cmdutil.ParseEnvironmentArguments(envVars)
		if len(errs) > 0 {
			return nil, errors.NewAggregate(errs)
		}
		return genapp.Environment(env), nil
	}
	return genapp.Environment{}, nil
}

type sourceRefGenerator interface {
	FromGitURL(url string) (*genapp.SourceRef, error)
	FromDirectory(dir string) (*genapp.SourceRef, error)
}

type strategyRefGenerator interface {
	FromSourceRefAndDockerContext(srcRef *genapp.SourceRef, dockerContext string) (*genapp.BuildStrategyRef, error)
	FromSTIBuilderImage(builderRef *genapp.ImageRef) (*genapp.BuildStrategyRef, error)
	FromSourceRef(srcRef *genapp.SourceRef) (*genapp.BuildStrategyRef, error)
}

type imageRefGenerator interface {
	FromNameAndResolver(builderImage string, resolver genapp.Resolver) (*genapp.ImageRef, error)
}

type appGenerator struct {
	input          params
	resolver       genapp.Resolver
	srcRefGen      sourceRefGenerator
	strategyRefGen strategyRefGenerator
	imageRefGen    imageRefGenerator
}

func (g *appGenerator) generateSourceRef() (*genapp.SourceRef, error) {
	var result *genapp.SourceRef
	var err error
	if len(g.input.sourceURL) > 0 {
		glog.V(3).Infof("Generating sourceRef from Git URL: %s", g.input.sourceURL)
		if result, err = g.srcRefGen.FromGitURL(g.input.sourceURL); err != nil {
			glog.V(3).Infof("Error received while generating source reference: %#v", err)
			return nil, err
		}
	} else {
		glog.V(3).Infof("Generating sourceRef from directory: %s", g.input.sourceDir)
		if result, err = g.srcRefGen.FromDirectory(g.input.sourceDir); err != nil {
			glog.V(3).Infof("Error received while generating source reference: %#v", err)
			return nil, err
		}
	}
	if len(g.input.sourceRef) > 0 {
		glog.V(3).Infof("Setting sourceRef reference to %s", g.input.sourceRef)
		result.Ref = g.input.sourceRef
	}
	if len(g.input.name) > 0 {
		glog.V(3).Infof("Setting sourceRef name to %s", g.input.name)
		result.Name = g.input.name
	}
	return result, nil
}

func (g *appGenerator) generateBuildStrategyRef(srcRef *genapp.SourceRef) (*genapp.BuildStrategyRef, error) {
	var strategyRef *genapp.BuildStrategyRef
	var err error
	if len(g.input.dockerContext) > 0 {
		glog.V(3).Infof("Generating build strategy reference using dockerContext: %s", g.input.dockerContext)
		strategyRef, err = g.strategyRefGen.FromSourceRefAndDockerContext(srcRef, g.input.dockerContext)
		if err != nil {
			return nil, err
		}
	} else if len(g.input.builderImage) > 0 {
		glog.V(3).Infof("Generating build strategy reference using builder image: %s", g.input.builderImage)
		builderRef, err := g.imageRefGen.FromNameAndResolver(g.input.builderImage, g.resolver)
		if err != nil {
			return nil, err
		}
		strategyRef, err = g.strategyRefGen.FromSTIBuilderImage(builderRef)
		if err != nil {
			return nil, err
		}
	} else {
		glog.V(3).Infof("Detecting build strategy using source reference: %#v", srcRef)
		strategyRef, err = g.strategyRefGen.FromSourceRef(srcRef)
		if err != nil {
			return nil, err
		}
	}
	if len(g.input.port) > 0 {
		strategyRef.Base.Info.Config.ExposedPorts = map[string]struct{}{g.input.port: {}}
	}
	return strategyRef, nil

}

func (g *appGenerator) generatePipeline(srcRef *genapp.SourceRef, strategyRef *genapp.BuildStrategyRef) (*genapp.Pipeline, error) {
	pipeline, err := genapp.NewBuildPipeline(srcRef.Name, strategyRef.Base, strategyRef, srcRef)
	if err != nil {
		return nil, err
	}
	if err := pipeline.NeedsDeployment(g.input.env); err != nil {
		return nil, err
	}

	return pipeline, nil
}

func (g *appGenerator) run() (*kapi.List, error) {
	// Get a SourceRef
	glog.V(3).Infof("About to generate source reference with input: %#v", g.input)
	srcRef, err := g.generateSourceRef()
	if err != nil {
		return nil, err
	}
	glog.V(2).Infof("Source reference: %#v", srcRef)

	// Get a BuildStrategyRef
	strategyRef, err := g.generateBuildStrategyRef(srcRef)
	if err != nil {
		return nil, err
	}
	glog.V(2).Infof("Generated build strategy reference: %#v", strategyRef)

	// Get a build pipeline
	pipeline, err := g.generatePipeline(srcRef, strategyRef)
	if err != nil {
		return nil, err
	}
	glog.V(2).Infof("Generated pipeline: %#v", pipeline)

	// Generate objects and service
	objects, err := pipeline.Objects(genapp.NewAcceptFirst())
	if err != nil {
		return nil, err
	}
	objects = genapp.AddServices(objects)

	return &kapi.List{Items: objects}, nil
}

func checkErr(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
