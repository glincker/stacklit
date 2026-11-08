package detect

import "sort"

// FileEnvVars returns the env var names referenced in one source file, so
// callers can scan files one at a time without holding them all in memory.
func FileEnvVars(content []byte) []string {
	src := string(content)
	seen := map[string]bool{}
	for _, re := range envPatterns {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			if !commonEnvVars[m[1]] {
				seen[m[1]] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
