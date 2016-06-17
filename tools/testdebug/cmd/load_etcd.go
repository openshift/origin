package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	coreosetcdclient "github.com/coreos/etcd/client"
	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/etcd/etcdserver"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/server/start"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

const RecommendedLoadEtcdDumpName = "start-api"

type DebugAPIServerOptions struct {
	Out io.Writer

	EtcdDumpFile string
	AllowAll     bool
}

func NewDebugAPIServerCommand() *cobra.Command {
	o := &DebugAPIServerOptions{Out: os.Stdout}

	cmd := &cobra.Command{
		Use:   RecommendedLoadEtcdDumpName + " etcd_dump.json",
		Short: "Start API server using etcddump",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(args))

			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.AllowAll, "allow-all", true, "change policy to grant system:authenticated cluster-admin powers")

	flagtypes.GLog(cmd.PersistentFlags())

	return cmd
}

func (o *DebugAPIServerOptions) Complete(args []string) error {
	if len(args) != 1 {
		return errors.New("etcd_dump.json file is required")
	}

	o.EtcdDumpFile = args[0]

	return nil
}

func (o *DebugAPIServerOptions) Run() error {
	masterConfig, err := testserver.DefaultMasterOptionsWithTweaks(true /*start etcd server*/, true /*use default ports*/)
	if err != nil {
		return err
	}

	etcdConfig := masterConfig.EtcdConfig
	masterConfig.EtcdConfig = nil
	masterConfig.DNSConfig = nil

	etcdserver.RunEtcd(etcdConfig)

	if err := o.ImportEtcdDump(masterConfig.EtcdClientInfo); err != nil {
		return err
	}

	if err := o.StartAPIServer(*masterConfig); err != nil {
		return err
	}

	if o.AllowAll {
		osClient, err := testutil.GetClusterAdminClient(testutil.GetBaseDir() + "/openshift.local.config/master/admin.kubeconfig")
		if err != nil {
			return err
		}

		addClusterAdmin := &policy.RoleModificationOptions{
			RoleName:            bootstrappolicy.ClusterAdminRoleName,
			RoleBindingAccessor: policy.ClusterRoleBindingAccessor{Client: osClient},
			Groups:              []string{"system:authenticated"},
		}
		if err := addClusterAdmin.AddRole(); err != nil {
			return err
		}
	}

	select {}
}

func (o *DebugAPIServerOptions) StartAPIServer(masterConfig configapi.MasterConfig) error {
	openshiftConfig, err := origin.BuildMasterConfig(masterConfig)
	if err != nil {
		return err
	}

	kubeMasterConfig, err := start.BuildKubernetesMasterConfig(openshiftConfig)
	if err != nil {
		return err
	}

	fmt.Printf("Starting master on %s\n", masterConfig.ServingInfo.BindAddress)
	fmt.Printf("Public master address is %s\n", masterConfig.AssetConfig.MasterPublicURL)
	return start.StartAPI(openshiftConfig, kubeMasterConfig)
}

func (o *DebugAPIServerOptions) ImportEtcdDump(etcdClientInfo configapi.EtcdConnectionInfo) error {
	infile, err := os.Open(o.EtcdDumpFile)
	if err != nil {
		return err
	}
	etcdDump := &coreosetcdclient.Response{}
	if err := json.NewDecoder(infile).Decode(etcdDump); err != nil {
		return err
	}

	// Connect and setup etcd interfaces
	etcdClient, err := etcd.GetAndTestEtcdClient(etcdClientInfo)
	if err != nil {
		return err
	}

	nodeList := []*coreosetcdclient.Node{}
	nodeList = append(nodeList, etcdDump.Node)
	for i := 0; i < len(nodeList); i++ {
		node := nodeList[i]
		if node == nil {
			continue
		}

		for j := range node.Nodes {
			nodeList = append(nodeList, node.Nodes[j])
		}
		if len(node.Key) == 0 {
			continue
		}

		if node.Dir {
			if _, err := etcdClient.CreateDir(node.Key, uint64(0)); err != nil {
				return err
			}
			continue
		}

		if _, err := etcdClient.Create(node.Key, node.Value, uint64(0)); err != nil {
			return err
		}
	}

	return nil
}
