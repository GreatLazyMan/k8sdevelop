package options

import "github.com/GreatLazyMan/simplecsi/pkg/hostpath"

var Opts Options

type Options struct {
	Endpoint string
	Config   hostpath.Config
}
