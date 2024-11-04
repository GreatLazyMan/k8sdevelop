package main

import (
	"flag"

	"github.com/GreatLazyMan/simplecsi/pkg/kvm"

	"k8s.io/klog/v2"
)

var (
	endpoint string
	nodeID   string
)

func main() {
	flag.StringVar(&endpoint, "endpoint", "", "CSI Endpoint")
	flag.StringVar(&nodeID, "nodeid", "", "node id")

	klog.InitFlags(nil)
	flag.Parse()

	d := kvm.NewDriver(nodeID, endpoint)
	d.Run()
}
