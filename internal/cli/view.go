package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/renderer"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "View the generated stacklit.json index",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile("stacklit.json")
		if err != nil {
			return fmt.Errorf("reading stacklit.json: %w (run `stacklit generate` first)", err)
		}

		var idx schema.Index
		if err := json.Unmarshal(data, &idx); err != nil {
			return fmt.Errorf("parsing stacklit.json: %w", err)
		}

		htmlPath := "stacklit.html"
		var err2 error
		if g, gerr := graph.Load(graphFile); gerr == nil {
			err2 = renderer.WriteHTMLWithGraph(&idx, g, htmlMaxNodes(), htmlPath)
		} else {
			err2 = renderer.WriteHTML(&idx, htmlPath)
		}
		if err := err2; err != nil {
			return fmt.Errorf("writing HTML: %w", err)
		}

		fmt.Println("Opening visual map...")
		openBrowser(htmlPath)
		return nil
	},
}
