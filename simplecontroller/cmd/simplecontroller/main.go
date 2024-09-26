package main

import (
	"os"

	"k8s.io/component-base/cli"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/GreatLazyMan/simplecontroller/cmd/simplecontroller/app"
)

func main() {
	ctx := ctrl.SetupSignalHandler()
	cmd := app.NewSimplecontrollerCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
