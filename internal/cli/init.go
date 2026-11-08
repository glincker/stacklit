package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/glincker/stacklit/internal/engine"
	"github.com/spf13/cobra"
)

var (
	initHook      bool
	initWorkspace string
	initSummary   bool
	initMultiFile string
	initMultiOut  string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize stacklit in the current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if initMultiFile != "" {
			result, err := engine.RunMulti(engine.MultiOptions{
				ReposFile: initMultiFile,
				Output:    initMultiOut,
				Quiet:     false,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Multi-repo index: %s (%d repos, %d failed)\n", result.OutputPath, result.RepoCount, result.Failed)
			return nil
		}

		root := "."
		if initWorkspace != "" {
			root = initWorkspace
		}
		result, err := engine.Run(engine.Options{
			Root:        root,
			Workspace:   initWorkspace,
			InstallHook: initHook,
			Summary:     initSummary,
		})
		if err != nil {
			return err
		}
		fmt.Println("\nOpening visual map...")
		openBrowser(result.HTMLPath)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initHook, "hook", false, "Install a git post-commit hook")
	initCmd.Flags().StringVar(&initWorkspace, "workspace", "", "Path to workspace root (default: current directory)")
	initCmd.Flags().BoolVar(&initSummary, "summary", false, "Generate a high-level summary during init")
	initCmd.Flags().StringVar(&initMultiOut, "multi-output", "", "Where to write the multi-repo index (default stacklit-multi.json)")
	initCmd.Flags().StringVar(&initMultiFile, "multi", "", "Path to file listing repos for polyrepo scanning")
}

// openBrowser opens url in the default system browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		fmt.Printf("Open %s in your browser\n", url)
		return
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("Open %s in your browser\n", url)
	}
}
