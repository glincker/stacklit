package mcp

import (
	"os"
	"strings"

	"github.com/glincker/stacklit/internal/config"
)

// Server instructions ride the initialize result, so a model reads them before
// its first tool call. Kept short: they are paid on every session. The shipped
// wording routes exact names to grep or impact and asks for a neighbour check
// on multi-file answers; the strict no-crawl wording stays behind steering "d".
const instructionsFull = "Stacklit's index and symbol graph show this repo's structure. " +
	"Use query when you do not know where something is, and explain or path to see how things connect. " +
	"When you know the exact name, use grep or impact. " +
	"When the answer is several files, check the neighbours before answering: one query is a lead, not the answer. " +
	"Read code with slice, not whole files. If a result is stale or a symbol is missing, refresh the index."

const instructionsMinimal = "Stacklit's index and symbol graph show this repo's structure. " +
	"Use query when you do not know where something is. When you know the exact name, use grep or impact. " +
	"When the answer is several files, check the neighbours with map and slice before answering: one query is a lead, not the answer. " +
	"Read code with slice, not whole files. If a result is stale or a symbol is missing, refresh the index."

const instructionsStrictFull = "Stacklit's index and symbol graph are the source of truth for this repo's structure. " +
	"Use query, explain, impact and path before searching or reading files. " +
	"Read code with slice, not whole files. Do not crawl with ls -R, find or repo wide grep. " +
	"If a result is stale or a symbol is missing, refresh the index."

const instructionsStrictMinimal = "Stacklit's index and symbol graph are the source of truth for this repo's structure. " +
	"Use query, impact and map before searching or reading files. " +
	"Read code with slice, not whole files. Do not crawl with ls -R, find or repo wide grep. " +
	"If a result is stale or a symbol is missing, refresh the index."

// steeringVariant reads the steering choice the way setup does: the
// STACKLIT_STEERING variable first, then the config key of the working tree.
func steeringVariant() string {
	if v := os.Getenv("STACKLIT_STEERING"); v != "" {
		return v
	}
	return config.Load(".").Steering
}

// Instructions returns the handshake text for a profile under the configured steering.
func Instructions(profile string) string { return InstructionsFor(profile, steeringVariant()) }

// InstructionsFor returns the handshake text for a profile and steering variant.
func InstructionsFor(profile, steering string) string {
	strict := strings.EqualFold(strings.TrimSpace(steering), "d")
	switch {
	case profile == ProfileMinimal && strict:
		return instructionsStrictMinimal
	case profile == ProfileMinimal:
		return instructionsMinimal
	case strict:
		return instructionsStrictFull
	}
	return instructionsFull
}
