package parser

import (
	"os"
	"regexp"
	"strconv"
)

// defaultDataRunLines is the length of a run of literal key/value lines above
// which the run is blanked before parsing. Override with STACKLIT_DATA_RUN_LINES
// (0 turns it off).
const defaultDataRunLines = 120

var rubyDataLine = regexp.MustCompile(`^\s*(?:"[^"\n]*"|'[^'\n]*'|:\w+)\s*=>\s*(?:"[^"\n]*"|'[^'\n]*'|:\w+|\d+)\s*,?\s*(?:#.*)?$`)

func dataRunLimit() int {
	if v := os.Getenv("STACKLIT_DATA_RUN_LINES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return defaultDataRunLines
}

// maskRubyDataRuns blanks long runs of `"key" => "value",` lines (MIME tables
// and the like). The Ruby grammar parses such a hash in super-linear time, a
// 600 entry table costs seconds, and the entries hold no symbols. Bytes become
// spaces and newlines stay, so line numbers and offsets are unchanged.
func maskRubyDataRuns(src []byte) []byte {
	limit := dataRunLimit()
	if limit == 0 || len(src) < limit*4 {
		return src
	}
	type span struct{ start, end int }
	var runs []span
	runStart, runEnd, runLines := -1, 0, 0
	flush := func() {
		if runLines >= limit {
			runs = append(runs, span{runStart, runEnd})
		}
		runStart, runLines = -1, 0
	}
	for pos := 0; pos < len(src); {
		end := pos
		for end < len(src) && src[end] != '\n' {
			end++
		}
		if rubyDataLine.Match(src[pos:end]) {
			if runStart < 0 {
				runStart = pos
			}
			runEnd = end
			runLines++
		} else {
			flush()
		}
		pos = end + 1
	}
	flush()
	if len(runs) == 0 {
		return src
	}
	out := append([]byte(nil), src...)
	for _, r := range runs {
		for i := r.start; i < r.end; i++ {
			if out[i] != '\n' {
				out[i] = ' '
			}
		}
	}
	return out
}
