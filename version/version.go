package version

import (
	"runtime/debug"
)

var GitCommit = func() (commit string) {
	commit = "unknown"
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
			return
		}
	}
	return
}()
