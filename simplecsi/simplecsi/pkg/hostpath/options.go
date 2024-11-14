package hostpath

import (
	"fmt"
	"strconv"

	"github.com/GreatLazyMan/simplecsi/pkg/state"
)

// ignoreFailedReadParameterName is a parameter that, when set to true,
// causes the `--ignore-failed-read` option to be passed to `tar`.
const IgnoreFailedReadParameterName = "ignoreFailedRead"

func OptionsFromParameters(vol state.Volume, parameters map[string]string) ([]string, error) {
	// We do not support options for snapshots of block volumes
	if vol.VolAccessType == state.BlockAccess {
		return nil, nil
	}

	ignoreFailedReadString := parameters[IgnoreFailedReadParameterName]
	if len(ignoreFailedReadString) == 0 {
		return nil, nil
	}

	if ok, err := strconv.ParseBool(ignoreFailedReadString); err != nil {
		return nil, fmt.Errorf(
			"invalid value for %q, expected boolean but was %q",
			IgnoreFailedReadParameterName,
			ignoreFailedReadString,
		)
	} else if ok {
		return []string{"--ignore-failed-read"}, nil
	}

	return nil, nil
}
