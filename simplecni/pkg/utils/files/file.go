package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/GreatLazyMan/simplecni/pkg/constants"
)

func WriteSubnetFile(subnetMap map[string]string) error {
	dir, name := filepath.Split(constants.Path)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	tempFile := filepath.Join(dir, "."+name)
	f, err := os.Create(tempFile)
	defer f.Close()
	if err != nil {
		return fmt.Errorf("create subnetfile error: %v", err)
	}
	for k, v := range subnetMap {
		_, err := fmt.Fprintf(f, "%s=%s\n", strings.ToUpper(k), v)
		if err != nil {
			return fmt.Errorf("write %s/%s to subnetfile error: %v", k, v, err)
		}
	}

	// rename(2) the temporary file to the desired location so that it becomes
	// atomically visible with the contents
	return os.Rename(tempFile, constants.Path)
	// TODO - is this safe? What if it's not on the same FS?
}
