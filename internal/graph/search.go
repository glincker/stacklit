package graph

import (
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Hit is one ranked search result.
type Hit struct {
	Node  int
	Score float64
}

// aliases folds common abbreviations into one spelling on both the index and
// the query side, so js, javascript and ecma meet.
var aliases = map[string]string{
	"js": "javascript", "ecma": "javascript", "ts": "typescript", "py": "python", "rs": "rust",
	"md": "markdown", "cfg": "config", "conf": "config", "repo": "repository", "dir": "directory",
	"dirs": "directory", "env": "environment", "ctx": "context", "cmd": "command", "pkg": "package",
	"deps": "dependency", "dep": "dependencies", "db": "database", "doc": "documentation", "docs": "documentation",
}

// stem trims common English and code suffixes so parse, parser and parsing meet.
func stem(w string) string {
	if a, ok := aliases[w]; ok {
		w = a
	}
	for _, suf := range []string{"ing", "ers", "er", "ed", "es", "s", "e"} {
		if len(w) > len(suf)+2 && strings.HasSuffix(w, suf) {
			return w[:len(w)-len(suf)]
		}
	}
	return w
}

func stemAll(ws []string) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = stem(w)
	}
	return out
}

// searchIndex is a BM25F index over symbol name, signature, doc, body and path.
type searchIndex struct {
	tf     []map[string]float64 // weighted term frequency per node
	length []float64
	df     map[string]int
	avgLen float64
	n      int
}

// Field weights: a hit in a name outranks one in prose or code.
const (
	wName = 5.0
	wSig  = 1.0
	wDoc  = 1.5
	wBody = 1.0
	wDir  = 0.5
	wBase = 2.5
	wLang = 1.0
)

func (g *SymGraph) buildSearch() *searchIndex {
	ix := &searchIndex{tf: make([]map[string]float64, len(g.Nodes)), length: make([]float64, len(g.Nodes)), df: map[string]int{}}
	total := 0.0
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind == KindFile {
			continue
		}
		tf := map[string]float64{}
		add := func(text string, w float64) {
			for _, t := range stemAll(words(text)) {
				if stopWords[t] || len(t) < 2 {
					continue
				}
				tf[t] += w
				ix.length[i] += w
			}
		}
		add(n.Name, wName)
		add(n.Sig, wSig)
		add(n.Doc, wDoc)
		add(n.Body, wBody)
		add(filepath.Dir(n.File), wDir)
		add(strings.TrimSuffix(filepath.Base(n.File), filepath.Ext(n.File)), wBase)
		add(n.Lang, wLang)
		ix.tf[i] = tf
		for t := range tf {
			ix.df[t]++
		}
		total += ix.length[i]
		ix.n++
	}
	if ix.n > 0 {
		ix.avgLen = total / float64(ix.n)
	}
	return ix
}

func (g *SymGraph) searchIdx() *searchIndex {
	if g.sx == nil || len(g.sx.tf) != len(g.Nodes) {
		g.sx = g.buildSearch()
	}
	return g.sx
}

// bm25 scores every node against stemmed query terms.
func (ix *searchIndex) bm25(terms []string) map[int]float64 {
	k1, b := envFloat("STACKLIT_BM25_K1", 1.2), envFloat("STACKLIT_BM25_B", 0.75)
	cov := envFloat("STACKLIT_COVERAGE_BONUS", 0.5)
	out := map[int]float64{}
	for i, tf := range ix.tf {
		if tf == nil {
			continue
		}
		s := 0.0
		matched := 0
		for _, t := range terms {
			f := tf[t]
			if f == 0 {
				continue
			}
			matched++
			idf := math.Log(1 + (float64(ix.n)-float64(ix.df[t])+0.5)/(float64(ix.df[t])+0.5))
			s += idf * f * (k1 + 1) / (f + k1*(1-b+b*ix.length[i]/ix.avgLen))
		}
		if matched > 0 {
			// Covering more of the question's terms beats one strong term.
			out[i] = s * (1 + cov*float64(matched-1))
		}
	}
	return out
}

var quotedRe = regexp.MustCompile(`"([^"]+)"`)

// parsedQuery separates quoted phrases from loose words.
type parsedQuery struct {
	raw     string
	phrases []string // lowercase
	terms   []string // stemmed, stop words removed, phrases included
}

func parseQuery(q string) parsedQuery {
	pq := parsedQuery{raw: strings.TrimSpace(q)}
	for _, m := range quotedRe.FindAllStringSubmatch(q, -1) {
		if p := strings.ToLower(strings.TrimSpace(m[1])); p != "" {
			pq.phrases = append(pq.phrases, p)
		}
	}
	seen := map[string]bool{}
	for _, w := range queryTerms(strings.ReplaceAll(q, `"`, " ")) {
		s := stem(w)
		if !seen[s] {
			seen[s] = true
			pq.terms = append(pq.terms, s)
		}
	}
	return pq
}

func canon(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(s))
}

var identRe = regexp.MustCompile("^[A-Za-z_$][A-Za-z0-9_$]*(?:(?:\\.|::|#|/)[A-Za-z_$][A-Za-z0-9_$]*)*$")

// cleanIdent strips quoting and call syntax from an identifier-looking query.
func cleanIdent(q string) string {
	q = strings.TrimSpace(q)
	q = strings.Trim(q, "`'\"")
	q = strings.TrimSuffix(q, "()")
	return strings.TrimSpace(q)
}

// IsIdentifierQuery reports whether q is a single code identifier (camelCase,
// snake_case, Type.Method, pkg::name, schema.table) rather than prose.
func IsIdentifierQuery(q string) bool {
	q = cleanIdent(q)
	if q == "" || strings.ContainsAny(q, " \t") || !identRe.MatchString(q) {
		return false
	}
	return true
}

// identHits resolves an identifier query by tier: exact name, exact ignoring
// case and underscores, ID or schema-prefixed suffix, then prefix. Earlier
// tiers get higher scores. Returns nil when q is not identifier shaped.
func (g *SymGraph) identHits(q string) []Hit {
	if !IsIdentifierQuery(q) {
		return nil
	}
	q = cleanIdent(q)
	var hits []Hit
	have := map[int]bool{}
	push := func(ids []int, score float64, limit int) {
		ids = g.rankByDegree(append([]int(nil), ids...))
		for _, id := range ids {
			if limit == 0 {
				return
			}
			if have[id] || g.Nodes[id].Kind == KindFile {
				continue
			}
			have[id] = true
			hits = append(hits, Hit{id, score})
			limit--
		}
	}
	limit := envInt("STACKLIT_IDENT_MAX", 12)
	push(g.byName[q], 4, limit)
	if i, ok := g.byID[q]; ok {
		push([]int{i}, 4, limit)
	}
	// A schema or package prefix (public.user_accounts, pkg::Name) falls back
	// to its last segment, and snake_case matches camelCase spellings.
	parts := regexp.MustCompile(`\.|::|#|/`).Split(q, -1)
	last := parts[len(parts)-1]
	cq, cl := canon(q), canon(last)
	var canonEq, qual, prefix []int
	for _, k := range g.nameKeys {
		ck := canon(k)
		switch {
		case ck == cq || ck == cl:
			canonEq = append(canonEq, g.byName[k]...)
		case len(cl) >= 3 && strings.HasPrefix(ck, cl):
			prefix = append(prefix, g.byName[k]...)
		}
	}
	if len(parts) > 1 {
		for i := range g.Nodes {
			if g.Nodes[i].Kind != KindFile && strings.HasSuffix(g.Nodes[i].ID, "::"+q) {
				qual = append(qual, i)
			}
		}
	}
	push(qual, 3.8, limit)
	push(canonEq, 3.5, limit)
	push(prefix, 2.5, limit)
	return hits
}

// Search ranks symbols for a question: BM25F over name, signature, doc, body
// and path, blended with PageRank, with identifier and quoted-phrase intents
// handled first. Order is deterministic.
func (g *SymGraph) Search(q string, limit int) []Hit {
	pq := parseQuery(q)
	score := map[int]float64{}
	for _, h := range g.identHits(q) {
		score[h.Node] = h.Score
	}
	ident := len(score) > 0
	if len(pq.terms) > 0 && !(ident && IsIdentifierQuery(q) && len(score) >= 3) {
		bm := g.searchIdx().bm25(pq.terms)
		maxBM := 0.0
		for _, s := range bm {
			if s > maxBM {
				maxBM = s
			}
		}
		pr := g.PageRank()
		w := envFloat("STACKLIT_PAGERANK_WEIGHT", 0.15)
		for i, s := range bm {
			v := s/maxBM + w*pr[i]
			if v > score[i] && score[i] < 2 {
				score[i] = v + score[i]/10
			}
		}
	}
	if len(pq.phrases) > 0 {
		g.applyPhrases(pq.phrases, score)
	}
	ids := make([]int, 0, len(score))
	for i := range score {
		ids = append(ids, i)
	}
	g.sortByScore(ids, func(i int) float64 { return score[i] })
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	hits := make([]Hit, len(ids))
	for i, id := range ids {
		hits[i] = Hit{id, score[id]}
	}
	return hits
}

// applyPhrases boosts nodes whose text holds a quoted phrase verbatim and, when
// any node does, drops the rest so a quote acts as a filter.
func (g *SymGraph) applyPhrases(phrases []string, score map[int]float64) {
	boost := envFloat("STACKLIT_PHRASE_BOOST", 2)
	matched := map[int]bool{}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind == KindFile {
			continue
		}
		text := strings.ToLower(n.Name + " " + n.Sig + " " + n.Doc + " " + n.File)
		cn := canon(text)
		for _, p := range phrases {
			if strings.Contains(text, p) || strings.Contains(cn, canon(p)) {
				matched[i] = true
				score[i] += boost
			}
		}
	}
	if len(matched) == 0 {
		return
	}
	for i := range score {
		if !matched[i] {
			delete(score, i)
		}
	}
}

// SearchFiles ranks files by their best symbols, for callers that want paths.
func (g *SymGraph) SearchFiles(q string, limit int) []string {
	type fs struct {
		file string
		s    float64
	}
	best := map[string][]float64{}
	for _, h := range g.Search(q, 0) {
		f := g.Nodes[h.Node].File
		best[f] = append(best[f], h.Score)
	}
	var list []fs
	for f, ss := range best {
		sort.Sort(sort.Reverse(sort.Float64Slice(ss)))
		tot := ss[0]
		for i := 1; i < len(ss) && i < 3; i++ {
			tot += 0.1 * ss[i]
		}
		list = append(list, fs{f, tot})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].s != list[j].s {
			return list[i].s > list[j].s
		}
		return list[i].file < list[j].file
	})
	var out []string
	for _, x := range list {
		out = append(out, x.file)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
