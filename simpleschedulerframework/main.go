package main

import (
	"os"

	_ "github.com/GreatLazyMan/simplescheduler/pkg/apis/config/scheme"
	"github.com/GreatLazyMan/simplescheduler/pkg/plugins"
	"k8s.io/component-base/cli"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
)

func main() {

	command := app.NewSchedulerCommand(
		app.WithPlugin(plugins.Name, plugins.New),
	)

	logs.InitLogs()
	defer logs.FlushLogs()

	code := cli.Run(command)
	os.Exit(code)

	//if err := command.Execute(); err != nil {
	//	_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
	//	os.Exit(1)
	//}

}
