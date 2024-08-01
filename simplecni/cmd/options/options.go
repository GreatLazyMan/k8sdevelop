package options

import "context"

type CmdLineOpts struct {
	IpMasq     bool
	ConfigPath string
	KubeConfig string
	Cancel     context.CancelFunc
	Ctx        context.Context
}
