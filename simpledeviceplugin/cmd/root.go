/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/GreatLazyMan/simpledeviceplugin/cmd/app"
	"github.com/GreatLazyMan/simpledeviceplugin/cmd/options"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "simpledeviceplugin",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		run(&options.Opts)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func copypflag(dstFlagSet *flag.FlagSet, srcFlagSet *flag.FlagSet, name string) {
	dstFlagSet.Var(srcFlagSet.Lookup(name).Value, srcFlagSet.Lookup(name).Name, srcFlagSet.Lookup(name).Usage)
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	fss := rootCmd.Flags()
	appFlagSet := flag.NewFlagSet("app", flag.ExitOnError)
	appFlagSet.StringVar(&options.Opts.Name, "name", "defaultname", "name")
	appFlagSet.StringVar(&options.Opts.KubeconfigPath, "kubeconfig", "", "kubeconfig")
	appFlagSet.StringVar(&options.Opts.DevicePluginPath, "devicepluginpath", pluginapi.DevicePluginPath, "devicepluginpath")
	fss.AddGoFlagSet(appFlagSet)

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

func run(opt *options.Options) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	go app.Start(ctx, &wg, opt)

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
