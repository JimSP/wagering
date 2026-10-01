// Fault hooks exist only in explicitly instrumented test builds.
package failpoint

import (
	"os"
	"path/filepath"
)

func Hit(point string) { hit(point, os.Exit) }

func hit(point string, exit func(int)) {
	if os.Getenv("FAILPOINT") != point {
		return
	}
	dir := os.Getenv("FAILPOINT_DIR")
	if dir == "" {
		return
	}
	f, e := os.OpenFile(filepath.Join(dir, point), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if e != nil {
		return
	}
	_ = f.Close()
	exit(86)
}
