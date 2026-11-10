package normalize

import (
	"regexp"
	"slices"
	"sort"
	"strings"
)

var spacesRe = regexp.MustCompile(`[ \t\x{00a0}\x{200b}]+`)

// normText makes text and aliases comparable: upper case, dashes as spaces, one
// space between words, curly apostrophes straight. Newlines are kept.
func normText(s string) string {
	s = strings.ToUpper(s)
	s = strings.NewReplacer("-", " ", "–", " ", "’", "'").Replace(s)
	return strings.TrimSpace(spacesRe.ReplaceAllString(s, " "))
}

type aliasEntry struct {
	text  string // normalized
	key   string
	ascii bool // only ASCII letters/digits/space/punct: must stand alone
}

// matcher finds catalog aliases in text, longest alias first, never two on the same span.
type matcher struct{ entries []aliasEntry }

func newMatcher(aliases map[string][]string) *matcher {
	m := &matcher{}
	for key, list := range aliases {
		for _, a := range list {
			if n := normText(a); n != "" {
				m.entries = append(m.entries, aliasEntry{text: n, key: key, ascii: isASCII(n)})
			}
		}
	}
	sort.Slice(m.entries, func(i, j int) bool {
		if len(m.entries[i].text) != len(m.entries[j].text) {
			return len(m.entries[i].text) > len(m.entries[j].text)
		}
		return m.entries[i].key < m.entries[j].key
	})
	return m
}

type aliasHit struct {
	key        string
	start, end int
}

// find returns hits in normalized line s, in text order.
func (m *matcher) find(s string) []aliasHit {
	var hits []aliasHit
	taken := make([]bool, len(s))
	for _, e := range m.entries {
		for from := 0; from < len(s); {
			i := strings.Index(s[from:], e.text)
			if i < 0 {
				break
			}
			start, end := from+i, from+i+len(e.text)
			from = start + 1
			if e.ascii && (isWordByte(s, start-1) || isWordByte(s, end)) {
				continue
			}
			if anyTaken(taken, start, end) {
				// The same alias on several cards (a family name): every card gets the hit.
				if slices.ContainsFunc(hits, func(h aliasHit) bool { return h.start == start && h.end == end }) {
					hits = append(hits, aliasHit{e.key, start, end})
				}
				continue
			}
			for k := start; k < end; k++ {
				taken[k] = true
			}
			hits = append(hits, aliasHit{e.key, start, end})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].start < hits[j].start })
	return hits
}

// keys returns the distinct keys found, in text order.
func (m *matcher) keys(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, h := range m.find(s) {
		if !seen[h.key] {
			seen[h.key] = true
			out = append(out, h.key)
		}
	}
	return out
}

func anyTaken(taken []bool, start, end int) bool {
	for k := start; k < end; k++ {
		if taken[k] {
			return true
		}
	}
	return false
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func isWordByte(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	b := s[i]
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9'
}
