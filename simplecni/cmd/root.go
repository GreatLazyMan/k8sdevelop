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

	"github.com/GreatLazyMan/simplecni/cmd/options"
	"github.com/GreatLazyMan/simplecni/pkg/backend"

	"github.com/coreos/go-systemd/daemon"
	"github.com/spf13/cobra"
	log "k8s.io/klog/v2"
)

type NetworkBackend struct {
	Type string
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cmd",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

var (
	opts options.CmdLineOpts
)

func copypflag(dstFlagSet *flag.FlagSet, srcFlagSet *flag.FlagSet, name string) {
	dstFlagSet.Var(srcFlagSet.Lookup(name).Value, srcFlagSet.Lookup(name).Name, srcFlagSet.Lookup(name).Usage)
}

func init() {
	cniFlags := rootCmd.Flags()
	cniFlags.BoolVar(&opts.IpMasq, "ip-masq", true, "setup IP masquerade rule for traffic destined outside of overlay network")
	cniFlags.StringVar(&opts.ConfigPath, "configpath", "/etc/simplecni/net-conf.json", "the config json path")
	cniFlags.StringVar(&opts.KubeConfig, "kubeconfig", "", "kubernetes's kubeconfig")

	// add klog pflag into commandline pflag
	log.InitFlags(nil)
	klogFlagSet := flag.NewFlagSet("klog", flag.ExitOnError)
	//// Only copy the non file logging options from klog
	copypflag(klogFlagSet, flag.CommandLine, "v")
	copypflag(klogFlagSet, flag.CommandLine, "vmodule")
	copypflag(klogFlagSet, flag.CommandLine, "log_backtrace_at")
	cniFlags.AddGoFlagSet(klogFlagSet)
	// klog will log to tmp files by default. override so all entries
	// can flow into journald (if running under systemd)
	err := flag.Set("logtostderr", "true")
	if err != nil {
		log.Errorf("Can't set the logtostderr pflagL %v", err)
		os.Exit(1)
	}
}

// Execute adds all child commands to the root command and sets pflags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func shutdownHandler(ctx context.Context, sigs chan os.Signal, cancel context.CancelFunc) {
	// Wait for the context do be Done or for the signal to come in to shutdown.
	select {
	case <-ctx.Done():
		log.Info("Stopping shutdownHandler...")
	case <-sigs:
		// Call cancel on the context to close everything down.
		cancel()
		log.Info("shutdownHandler sent cancel signal...")
	}

	// Unregister to get default OS nuke behaviour in case we don't exit cleanly
	signal.Stop(sigs)
}

func run() error {
	// This is the main context that everything should run in.
	// All spawned goroutines should exit when cancel is called on this context.
	// Go routines spawned from main.go coordinate using a WaitGroup. This provides a mechanism to allow the shutdownHandler goroutine
	// to block until all the goroutines return . If those goroutines spawn other goroutines then they are responsible for
	// blocking and returning only when cancel() is called.
	ctx, cancel := context.WithCancel(context.Background())
	opts.Ctx = ctx
	opts.Cancel = cancel
	defer cancel()

	// do some init work
	// Register for SIGINT and SIGTERM
	log.Info("Installing signal handlers")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		shutdownHandler(ctx, sigs, cancel)
		wg.Done()
	}()

	// do some real work
	backend := backend.NewNetworkBackend(&opts)
	if backend == nil {
		log.Error("backend can't be inited, please check your config")
		os.Exit(1)
	}
	log.Info("Running backend.")
	wg.Add(1)
	go func() {
		backend.Run(ctx)
		cancel()
		log.Infof("backend done")
		wg.Done()
	}()

	// notify ready
	_, err := daemon.SdNotify(false, "READY=1")
	if err != nil {
		log.Errorf("Failed to notify systemd the message READY=1 %v", err)
	}
	log.Info("Waiting for all goroutines to exit")
	wg.Wait()

	// do some clear work
	// Block waiting for all the goroutines to finish.
	log.Info("Exiting cleanly...")
	os.Exit(0)

	return nil
}
