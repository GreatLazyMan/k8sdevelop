package main

import (
	"os"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"

	"github.com/greatlazyman/apiserver/pkg/cmd"
)

func main() {
	stopCh := genericapiserver.SetupSignalHandler()
	cmd := cmd.NewHelloServerCommand(stopCh)
	code := cli.Run(cmd)
	os.Exit(code)
}
