package hostpath

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Capacity simulates linear storage of certain types ("fast",
// "slow"). To calculate the amount of allocated space, the size of
// all currently existing volumes of the same kind is summed up.
//
// Available capacity is configurable with a command line flag
// -capacity <type>=<size> where <type> is a string and <size>
// is a quantity (1T, 1Gi). More than one of those
// flags can be used.
//
// The underlying map will be initialized if needed by Set,
// which makes it possible to define and use a Capacity instance
// without explicit initialization (`var capacity Capacity` or as
// member in a struct).
type Capacity map[string]resource.Quantity

// Set is an implementation of flag.Value.Set.
func (c *Capacity) Set(arg string) error {
	parts := strings.SplitN(arg, "=", 2)
	if len(parts) != 2 {
		return errors.New("must be of format <type>=<size>")
	}
	quantity, err := resource.ParseQuantity(parts[1])
	if err != nil {
		return err
	}

	// We overwrite any previous value.
	if *c == nil {
		*c = Capacity{}
	}
	(*c)[parts[0]] = quantity
	return nil
}

func (c *Capacity) String() string {
	return fmt.Sprintf("%v", map[string]resource.Quantity(*c))
}

func (c *Capacity) Type() string {
	return fmt.Sprintf("Capacity")
}

var _ flag.Value = &Capacity{}

// Enabled returns true if capacities are configured.
func (c *Capacity) Enabled() bool {
	return len(*c) > 0
}

// StringArray is a flag.Value implementation that allows to specify
// a comma-separated list of strings on the command line.
type StringArray []string

// Set is an implementation of flag.Value.Set.
func (s *StringArray) Set(value string) error {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		*s = append(*s, strings.TrimSpace(part))
	}
	return nil
}

// String is an implementation of flag.Value.String.
func (s *StringArray) String() string {
	return fmt.Sprintf("%v", []string(*s))
}

func (s *StringArray) Type() string {
	return "StringArray"
}

var _ flag.Value = &StringArray{}
