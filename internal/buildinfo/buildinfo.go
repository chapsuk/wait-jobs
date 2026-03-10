package buildinfo

import "fmt"

var (
	// These values are injected at build time via -ldflags.
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return fmt.Sprintf("version=%s commit=%s date=%s", Version, Commit, Date)
}
