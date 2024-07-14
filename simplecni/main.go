/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/GreatLazyMan/simplecni/cmd"
	log "k8s.io/klog/v2"
)

type CmdLineOpts struct {
	ipMasq bool
}

var (
	cniFlags = flag.NewFlagSet("simplecni", flag.ExitOnError)
	opts     CmdLineOpts
)

func copyFlag(name string) {
	cniFlags.Var(flag.Lookup(name).Value, flag.Lookup(name).Name, flag.Lookup(name).Usage)
}

func init() {
	cniFlags.BoolVar(&opts.ipMasq, "ip-masq", false, "setup IP masquerade rule for traffic destined outside of overlay network")
	// add klog flag into commandline flag
	log.InitFlags(nil)
	// klog will log to tmp files by default. override so all entries
	// can flow into journald (if running under systemd)
	err := flag.Set("logtostderr", "true")
	if err != nil {
		log.Errorf("Can't set the logtostderr flagL %v", err)
		os.Exit(1)
	}
	// Only copy the non file logging options from klog
	copyFlag("v")
	copyFlag("vmodule")
	copyFlag("log_backtrace_at")

	cniFlags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
		cniFlags.PrintDefaults()
		os.Exit(0)
	}

	err = cniFlags.Parse(os.Args[1:])

	if err != nil {
		log.Error("Can't parse cni flags", err)
		os.Exit(1)
	}
}

func main() {
	cmd.Execute()
}
