package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/glincker/stacklit/internal/ingest"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/spf13/cobra"
)

func init() { rootCmd.AddCommand(newIngestCmd()) }

func newIngestCmd() *cobra.Command {
	var docs, api, sch, owners, gitOwners, stdout bool
	var days int
	var ext extIngest
	var outPath string
	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Add docs, API specs, SQL schemas and ownership to a context sidecar",
		Long: `Collects non-code context into stacklit.context.json (stacklit.json is untouched).

Connectors (all run when none is selected):
  --docs         markdown docs, ADRs and RFCs: title, status, headings
  --api          OpenAPI/Swagger files: operations with ids and summaries
  --schema       SQL CREATE TABLE and Prisma models: tables and columns
  --owners       CODEOWNERS rules mapped onto modules from stacklit.json
  --git-owners   top recent commit authors per module (names only, opt-in)

Limits are read from STACKLIT_INGEST_MAX_BYTES and STACKLIT_INGEST_MAX_ITEMS.

External inputs (the connectors above are skipped when one is given):
  --adapter "<cmd>"   run a command that gets file paths on stdin and prints JSONL
                      nodes and edges; results are stored as INFERRED concepts with a
                      source field. Limits: STACKLIT_ADAPTER_TIMEOUT_SEC,
                      STACKLIT_ADAPTER_MAX_BYTES, STACKLIT_ADAPTER_MAX_ITEMS.
  --scip index.scip   upgrade INFERRED symbol graph edges to EXACT where the SCIP
                      index confirms them (rewrites the graph file in place)

Stacklit makes no network or model calls; an adapter is your own command.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if ext.active() {
				ext.outPath, ext.stdout = outPath, stdout
				return ext.run(cmd)
			}
			if !docs && !api && !sch && !owners && !gitOwners {
				docs, api, sch, owners = true, true, true, true
			}
			var modules []string
			if data, err := os.ReadFile("stacklit.json"); err == nil {
				var idx schema.Index
				if json.Unmarshal(data, &idx) == nil {
					for m := range idx.Modules {
						modules = append(modules, m)
					}
					sort.Strings(modules)
				}
			}
			ctx, err := ingest.Run(ingest.Options{
				Root: ".", Docs: docs, API: api, Schema: sch,
				Owners: owners, GitOwners: gitOwners, GitDays: days, Modules: modules,
			})
			if err != nil {
				return err
			}
			ingest.CarryAdapterOutput(ctx, outPath)
			b, err := json.MarshalIndent(ctx, "", "  ")
			if err != nil {
				return err
			}
			b = append(b, '\n')
			if stdout {
				_, err = cmd.OutOrStdout().Write(b)
				return err
			}
			if err := os.WriteFile(outPath, b, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d docs, %d apis, %d tables)\n", outPath, len(ctx.Docs), len(ctx.APIs), len(ctx.Tables))
			return nil
		},
	}
	f := cmd.Flags()
	f.BoolVar(&docs, "docs", false, "Index markdown docs, ADRs and RFCs")
	f.BoolVar(&api, "api", false, "Index OpenAPI/Swagger operations")
	f.BoolVar(&sch, "schema", false, "Index SQL and Prisma tables")
	f.BoolVar(&owners, "owners", false, "Map CODEOWNERS onto modules")
	f.BoolVar(&gitOwners, "git-owners", false, "Add top recent commit authors per module")
	f.IntVar(&days, "days", 0, "History window for --git-owners (default 180, env STACKLIT_OWNERS_DAYS)")
	f.StringVarP(&outPath, "output", "o", ingest.ContextFile, "Output file")
	ext.bind(cmd)
	f.BoolVar(&stdout, "stdout", false, "Print to stdout instead of writing a file")
	return cmd
}
