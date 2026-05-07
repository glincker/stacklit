package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/glincker/stacklit/internal/renderer"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	var format, source, output string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the index in a human-readable format",
		Long: `Renders stacklit.json in a different format for sharing.

Currently supports markdown, which produces a readable project overview
suitable for pasting into PR descriptions, GitHub issues, or chat.

Examples:
  stacklit export                          Print markdown to stdout
  stacklit export --format md              Same as above
  stacklit export -o stacklit.md           Write markdown to a file
  stacklit export --source ./other.json    Read from a non-default index`,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(source)
			if err != nil {
				return fmt.Errorf("could not read %s: %w (run 'stacklit generate' first, or 'stacklit init')", source, err)
			}
			var idx schema.Index
			if err := json.Unmarshal(data, &idx); err != nil {
				return fmt.Errorf("could not parse %s: %w", source, err)
			}

			switch format {
			case "md", "markdown":
				if output == "" {
					fmt.Print(renderer.RenderMarkdown(&idx))
					return nil
				}
				if err := renderer.WriteMarkdown(&idx, output); err != nil {
					return fmt.Errorf("could not write %s: %w", output, err)
				}
				fmt.Fprintf(cmd.OutOrStderr(), "Wrote %s\n", output)
				return nil
			default:
				return fmt.Errorf("unknown format %q (supported: md, markdown)", format)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "md", "Output format (md)")
	cmd.Flags().StringVar(&source, "source", "stacklit.json", "Path to the index JSON")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write to file instead of stdout")
	return cmd
}
