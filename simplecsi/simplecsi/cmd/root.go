/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
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
	fss.StringVarP(&options.Opts.Endpoint, "endpoint", "e", "unix://tmp/csi.sock", "CSI endpoint")
}

func run(opt *options.Options) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	serverConfig := server.NewServerconfig(opt.Endpoint)
	wg.Add(1)
	go server.Serve(&serverConfig, ctx, &wg)

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
