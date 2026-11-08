package mcp

// Server instructions ride the initialize result, so a model reads them before
// its first tool call. Kept short: they are paid on every session.
const instructionsFull = "Stacklit's index and symbol graph are the source of truth for this repo's structure. " +
	"Use query, explain, impact and path before searching or reading files. " +
	"Read code with slice, not whole files. Do not crawl with ls -R, find or repo wide grep. " +
	"If a result is stale or a symbol is missing, refresh the index."

const instructionsMinimal = "Stacklit's index and symbol graph are the source of truth for this repo's structure. " +
	"Use query, impact and map before searching or reading files. " +
	"Read code with slice, not whole files. Do not crawl with ls -R, find or repo wide grep. " +
	"If a result is stale or a symbol is missing, refresh the index."

// Instructions returns the handshake text for a profile.
func Instructions(profile string) string {
	if profile == ProfileMinimal {
		return instructionsMinimal
	}
	return instructionsFull
}
