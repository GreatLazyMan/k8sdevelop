package main

import (
	"os"

	_ "github.com/GreatLazyMan/simplescheduler/pkg/apis/config/scheme"
	"github.com/GreatLazyMan/simplescheduler/pkg/plugins/coschedule"
	"k8s.io/component-base/cli"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // for rest client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
)

func main() {
	command := app.NewSchedulerCommand(
		app.WithPlugin(coschedule.Name, coschedule.New),
	)

	logs.InitLogs()
	defer logs.FlushLogs()

	code := cli.Run(command)
	os.Exit(code)
}
