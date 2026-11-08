package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/glincker/stacklit/internal/derive"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/spf13/cobra"
)

var deriveInject string

func newDeriveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "derive",
		Short: "Generate a compact codebase navigation map",
		Long: `Generates a token-efficient navigation map from stacklit.json.

The map contains architecture, modules, dependencies, and hints in a few hundred tokens
(measured sizes are in the README).

Use --inject to automatically insert the map into agent config files:
  stacklit derive                  Print map to stdout
  stacklit derive --inject claude  Inject into CLAUDE.md
  stacklit derive --inject cursor  Inject into .cursorrules
  stacklit derive --inject aider   Inject into .aider.conf.yml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load stacklit.json.
			data, err := os.ReadFile("stacklit.json")
			if err != nil {
				return fmt.Errorf("could not read stacklit.json: %w (run 'stacklit init' first)", err)
			}
			var idx schema.Index
			if err := json.Unmarshal(data, &idx); err != nil {
				return fmt.Errorf("could not parse stacklit.json: %w", err)
			}

			if deriveInject == "" {
				// Print to stdout.
				fmt.Print(derive.CompactMap(&idx))
				return nil
			}

			// Inject into target file.
			block := derive.InjectableBlock(&idx)
			var target string
			switch deriveInject {
			case "claude":
				target = "CLAUDE.md"
			case "cursor":
				target = ".cursorrules"
			case "aider":
				target = ".aider.conf.yml"
			default:
				return fmt.Errorf("unknown inject target %q (use: claude, cursor, aider)", deriveInject)
			}

			if err := derive.InjectIntoFile(target, block); err != nil {
				return err
			}
			fmt.Printf("Injected codebase map into %s\n", target)
			return nil
		},
	}
	cmd.Flags().StringVar(&deriveInject, "inject", "", "Inject map into config file (claude, cursor, aider)")
	return cmd
}
