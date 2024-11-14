/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/GreatLazyMan/simplecsi/cmd/options"
	"github.com/GreatLazyMan/simplecsi/pkg/server"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "simplecsi",
	Short: "simple csi",
	Long:  `Simple csi`,
	Run: func(cmd *cobra.Command, args []string) {
		run(&options.Opts)
	},
}

func Parse(ep string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(ep), "unix://") || strings.HasPrefix(strings.ToLower(ep), "tcp://") {
		s := strings.SplitN(ep, "://", 2)
		if s[1] != "" {
			return s[0], s[1], nil
		}
		return "", "", fmt.Errorf("Invalid endpoint: %v", ep)
	}
	// Assume everything else is a file path for a Unix Domain Socket.
	return "unix", ep, nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	fss := rootCmd.Flags()
	hpFlagSet := flag.NewFlagSet("hostpath", flag.ExitOnError)
	hpFlagSet.StringVar(&options.Opts.Endpoint, "endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	//fss.StringVar(&options.Opts.Config.Endpoint, "endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	hpFlagSet.StringVar(&options.Opts.Config.DriverName, "drivername", "hostpath.csi.k8s.io", "name of the driver")
	hpFlagSet.StringVar(&options.Opts.Config.StateDir, "statedir", "/csi-data-dir", "directory for storing state information across driver restarts, volumes and snapshots")
	hpFlagSet.StringVar(&options.Opts.Config.NodeID, "nodeid", "", "node id")
	hpFlagSet.BoolVar(&options.Opts.Config.Ephemeral, "ephemeral", false, "publish volumes in ephemeral mode even if kubelet did not ask for it (only needed for Kubernetes 1.15)")
	hpFlagSet.Int64Var(&options.Opts.Config.MaxVolumesPerNode, "maxvolumespernode", 0, "limit of volumes per node")
	hpFlagSet.Var(&options.Opts.Config.Capacity, "capacity", "Simulate storage capacity. The parameter is <kind>=<quantity> where <kind> is the value of a 'kind' storage class parameter and <quantity> is the total amount of bytes for that kind. The flag may be used multiple times to configure different kinds.")
	hpFlagSet.BoolVar(&options.Opts.Config.EnableAttach, "enable-attach", false, "Enables RPC_PUBLISH_UNPUBLISH_VOLUME capability.")
	hpFlagSet.BoolVar(&options.Opts.Config.CheckVolumeLifecycle, "check-volume-lifecycle", false, "Can be used to turn some violations of the volume lifecycle into warnings instead of failing the incorrect gRPC call. Disabled by default because of https://github.com/kubernetes/kubernetes/issues/101911.")
	hpFlagSet.Int64Var(&options.Opts.Config.MaxVolumeSize, "max-volume-size", 1024*1024*1024*1024, "maximum size of volumes in bytes (inclusive)")
	hpFlagSet.BoolVar(&options.Opts.Config.EnableTopology, "enable-topology", true, "Enables PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS capability.")
	hpFlagSet.BoolVar(&options.Opts.Config.EnableVolumeExpansion, "node-expand-required", true, "Enables volume expansion capability of the plugin(Deprecated). Please use enable-volume-expansion flag.")
	hpFlagSet.BoolVar(&options.Opts.Config.EnableVolumeExpansion, "enable-volume-expansion", true, "Enables volume expansion feature.")
	hpFlagSet.BoolVar(&options.Opts.Config.EnableControllerModifyVolume, "enable-controller-modify-volume", false, "Enables Controller modify volume feature.")
	hpFlagSet.Var(&options.Opts.Config.AcceptedMutableParameterNames, "accepted-mutable-parameter-names", "Comma separated list of parameter names that can be modified on a persistent volume. This is only used when enable-controller-modify-volume is true. If unset, all parameters are mutable.")
	hpFlagSet.BoolVar(&options.Opts.Config.DisableControllerExpansion, "disable-controller-expansion", false, "Disables Controller volume expansion capability.")
	hpFlagSet.BoolVar(&options.Opts.Config.DisableNodeExpansion, "disable-node-expansion", false, "Disables Node volume expansion capability.")
	hpFlagSet.BoolVar(&options.Opts.Config.IsAgent, "isagent", false, "is agent")
	hpFlagSet.Int64Var(&options.Opts.Config.MaxVolumeExpansionSizeNode, "max-volume-size-node", 0, "Maximum allowed size of volume when expanded on the node. Defaults to same size as max-volume-size.")
	hpFlagSet.Int64Var(&options.Opts.Config.AttachLimit, "attach-limit", 0, "Maximum number of attachable volumes on a node. Zero refers to no limit.")
	hpFlagSet.StringVar(&options.Opts.Config.VendorVersion, "version", "0.0.1", "")
	hpFlagSet.StringVar(&options.Opts.Config.LinuxUtilsImage, "linux-utils-image",
		"aiops-8af5363b.ecis.huhehaote-1.cmecloud.cn/kosmos/openebs/linux-utils:3.3.0", "")
	fss.AddGoFlagSet(hpFlagSet)

	klogFlagSet := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(nil)
	copypflag(klogFlagSet, flag.CommandLine, "v")
	copypflag(klogFlagSet, flag.CommandLine, "vmodule")
	copypflag(klogFlagSet, flag.CommandLine, "log_backtrace_at")
	fss.AddGoFlagSet(klogFlagSet)
	err := flag.Set("logtostderr", "true")
	if err != nil {
		klog.Errorf("Can't set the logtostderr pflagL %v", err)
		os.Exit(1)
	}
}

func copypflag(dstFlagSet *flag.FlagSet, srcFlagSet *flag.FlagSet, name string) {
	dstFlagSet.Var(srcFlagSet.Lookup(name).Value, srcFlagSet.Lookup(name).Name, srcFlagSet.Lookup(name).Usage)
}

func run(opt *options.Options) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	serverConfig := server.NewServerconfig(opt.Endpoint)
	wg.Add(1)
	go server.Serve(&serverConfig, opt.Config, ctx, &wg)

	sigc := make(chan os.Signal, 1)
	sigs := []os.Signal{
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
	}
	signal.Notify(sigc, sigs...)

	<-sigc
	cancel()
	wg.Wait()
	klog.Info("Exit....")
}
