package cli

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// resolveVersion prefers the ldflags value, then the module version that
// `go install ...@vX.Y.Z` embeds, then the VCS revision, so binaries built
// without the Makefile never report a bare "dev".
func resolveVersion() string {
	if Version != "" && Version != "dev" {
		return strings.TrimPrefix(Version, "v")
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return strings.TrimPrefix(v, "v")
	}
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev != "" {
		if len(rev) > 7 {
			rev = rev[:7]
		}
		return "dev-" + rev + dirty
	}
	return "dev"
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the stacklit version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "stacklit %s\n", resolveVersion())
		},
	}
}

func init() {
	rootCmd.Version = resolveVersion()
	rootCmd.AddCommand(newVersionCmd())
}
