package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/git"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/glincker/stacklit/internal/walker"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show changes since last index generation",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Read stacklit.json
			data, err := os.ReadFile("stacklit.json")
			if err != nil {
				return fmt.Errorf("could not read stacklit.json: %w (run 'stacklit generate' first)", err)
			}

			var index schema.Index
			if err := json.Unmarshal(data, &index); err != nil {
				return fmt.Errorf("could not parse stacklit.json: %w", err)
			}

			storedHash := index.MerkleHash
			if storedHash == "" {
				return fmt.Errorf("stacklit.json has no merkle_hash; run 'stacklit generate' to rebuild")
			}

			// 2. Walk current source files, excluding Stacklit's own generated outputs.
			cfg := config.Load(".")
			files, err := walker.Walk(".", cfg.ScanIgnore())
			if err != nil {
				return fmt.Errorf("failed to walk source files: %w", err)
			}

			// 3. Read file contents and compute fresh Merkle hash
			contents := make(map[string][]byte, len(files))
			for _, f := range files {
				b, err := os.ReadFile(f)
				if err != nil {
					return fmt.Errorf("could not read %s: %w", f, err)
				}
				contents[f] = b
			}

			currentHash := git.ComputeMerkle(files, contents)

			// 4. Compare hashes
			if currentHash == storedHash {
				fmt.Println("Index is up to date. No source changes detected.")
				return nil
			}

			// 5. Hashes differ — report and suggest regeneration
			fmt.Println("Source files changed since last generation. Run 'stacklit generate' to update.")
			return nil
		},
	}
	return cmd
}
// comment
