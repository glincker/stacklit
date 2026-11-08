package main

import (
	"regexp"
)

// Cost is the size of some text an agent would have to read. Bytes drives the
// primary token estimate (bytes/4). Pieces is a second, independent estimate
// used only as a sensitivity check.
type Cost struct {
	Bytes  int
	Pieces int
}

var piecePattern = regexp.MustCompile(`[A-Za-z0-9_]+|[^\sA-Za-z0-9_]`)

// CostOf measures a string.
func CostOf(s string) Cost {
	return Cost{Bytes: len(s), Pieces: len(piecePattern.FindAllStringIndex(s, -1))}
}

// Add returns the sum of two costs.
func (c Cost) Add(o Cost) Cost {
	return Cost{Bytes: c.Bytes + o.Bytes, Pieces: c.Pieces + o.Pieces}
}

// Tokens is the primary approximation: ceil(bytes / 4).
func (c Cost) Tokens() int {
	return (c.Bytes + 3) / 4
}

// PieceTokens is the sensitivity estimate: word and punctuation pieces scaled
// by 1.3, a rough stand-in for BPE splitting of identifiers.
func (c Cost) PieceTokens() int {
	return int(float64(c.Pieces)*1.3 + 0.5)
}
