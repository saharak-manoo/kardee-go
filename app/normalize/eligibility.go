package normalize

import (
	"regexp"
	"slices"
	"strings"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/models"
)

// How promotion text names banks, networks and tiers (normalized, upper case).
var (
	bankWords = map[string][]string{
		"ktc":      {"KTC", "เคทีซี"},
		"ttb":      {"TTB", "ทีทีบี"},
		"krungsri": {"กรุงศรี", "KRUNGSRI"},
		"kbank":    {"กสิกร", "KBANK"},
	}
	networkWords = map[string][]string{
		"visa":       {"VISA", "วีซ่า"},
		"mastercard": {"MASTERCARD", "มาสเตอร์การ์ด"},
		"jcb":        {"JCB", "เจซีบี"},
		"unionpay":   {"UNIONPAY", "ยูเนี่ยนเพย์"},
		"amex":       {"AMERICAN EXPRESS", "AMEX"},
	}
	tierWords = map[string][]string{
		"classic":     {"CLASSIC", "คลาสสิก"},
		"gold":        {"GOLD", "โกลด์"},
		"platinum":    {"PLATINUM", "แพลทินัม"},
		"signature":   {"SIGNATURE", "ซิกเนเจอร์"},
		"infinite":    {"INFINITE", "อินฟินิท"},
		"titanium":    {"TITANIUM", "ไทเทเนียม", "ไทเทเนี่ยม"},
		"world-elite": {"WORLD ELITE", "เวิลด์ อีลิท"},
		"world":       {"WORLD", "เวิลด์"},
		"ultimate":    {"ULTIMATE", "อัลติเมท"},
		"diamond":     {"DIAMOND", "ไดมอนด์"},
		"green":       {"GREEN"},
	}

	issuedInThaiRe = regexp.MustCompile(`ที่ออกในประเทศไทย|ISSUED IN THAILAND`)
	exceptRe       = regexp.MustCompile(`ยกเว้น|EXCEPT`)
	onlyRe         = regexp.MustCompile(`เฉพาะ`)
)

func alt(words []string) string {
	q := make([]string, len(words))
	for i, w := range words {
		q[i] = regexp.QuoteMeta(w)
	}
	return strings.Join(q, "|")
}

// networkTierRe matches "<network> <tier>", e.g. "JCB PLATINUM".
var networkTierRe = func() *regexp.Regexp {
	var nets, tiers []string
	for _, w := range networkWords {
		nets = append(nets, w...)
	}
	for _, w := range tierWords {
		tiers = append(tiers, w...)
	}
	return regexp.MustCompile(`(` + alt(nets) + `) ?(` + alt(tiers) + `)`)
}()

func lookup(words map[string][]string, w string) string {
	for k, list := range words {
		if slices.Contains(list, w) {
			return k
		}
	}
	return ""
}

// ExtractEligibility reads which cards can use a promotion. cardTypes is structured
// card data from the bank (Krungsri); text is the promotion's title, description and
// conditions.
func ExtractEligibility(cat *catalog.Catalog, bank string, cardTypes []string, text string) models.Eligibility {
	norm := normText(text)

	// A network's own promotion: any bank's card of that network.
	if _, ok := cat.Networks[bank]; ok {
		return models.Eligibility{
			Rules:      []models.EligibilityRule{{Scope: models.ScopeNetwork, Network: bank, MinTier: lowestTier(cat, bank, norm)}},
			Confidence: models.ConfMedium,
		}
	}

	cards := map[string][]string{}
	for _, c := range cat.Cards {
		if c.Bank == bank {
			cards[c.Key] = c.Aliases
		}
	}
	m := newMatcher(cards)

	var e models.Eligibility
	if len(cardTypes) > 0 {
		var keys []string
		for _, name := range cardTypes {
			if k := exactAlias(m, name); k != "" {
				keys = appendNew(keys, k)
			} else {
				e.Unmatched = append(e.Unmatched, name)
			}
		}
		e.Confidence = models.ConfHigh
		if len(keys) > 0 {
			e.Rules = []models.EligibilityRule{{Scope: models.ScopeCards, CardKeys: keys}}
		} else {
			e.Rules = []models.EligibilityRule{{Scope: models.ScopeBankAll}}
			e.Confidence = models.ConfLow
		}
		return e
	}

	// "KTC JCB ทุกประเภท" / "all types of KTC JCB": every card of that bank and network.
	var bankNetRe *regexp.Regexp
	if words := bankWords[bank]; len(words) > 0 {
		b, n := alt(words), alt(allNetworkWords())
		bankNetRe = regexp.MustCompile(`(?:` + b + `) (` + n + `) ?ทุกประเภท|ALL TYPES OF (?:` + b + `) (` + n + `)`)
	}
	var onlyCards []string
	for _, line := range strings.Split(norm, "\n") {
		if loc := exceptRe.FindStringIndex(line); loc != nil {
			for _, k := range m.keys(line[loc[1]:]) {
				e.Excluded = appendNew(e.Excluded, k)
			}
			line = line[:loc[0]]
		}
		if issuedInThaiRe.MatchString(line) {
			if r, ok := networkRule(cat, line); ok {
				e.Rules = appendRule(e.Rules, r)
			}
		}
		if bankNetRe != nil {
			for _, sm := range bankNetRe.FindAllStringSubmatch(line, -1) {
				w := sm[1] + sm[2] // one of the two groups is set
				e.Rules = appendRule(e.Rules, models.EligibilityRule{Scope: models.ScopeNetwork, Network: lookup(networkWords, w), Issuer: bank})
			}
		}
		if loc := onlyRe.FindStringIndex(line); loc != nil {
			for _, k := range m.keys(line[loc[1]:]) {
				onlyCards = appendNew(onlyCards, k)
			}
		}
	}
	if len(onlyCards) > 0 {
		e.Rules = appendRule(e.Rules, models.EligibilityRule{Scope: models.ScopeCards, CardKeys: onlyCards})
	}
	if len(e.Rules) > 0 {
		e.Confidence = models.ConfMedium
		return e
	}
	e.Rules = []models.EligibilityRule{{Scope: models.ScopeBankAll}}
	e.Confidence = models.ConfLow
	bankAllRe := regexp.MustCompile(`บัตรเครดิต ?(?:` + alt(bankWords[bank]) + `)|ทุกประเภท|ทุกใบ`)
	if len(bankWords[bank]) > 0 && bankAllRe.MatchString(norm) || len(e.Excluded) > 0 {
		e.Confidence = models.ConfMedium
	}
	return e
}

// networkRule reads "JCB PLATINUM หรือ JCB ULTIMATE ที่ออกในประเทศไทย": the network and
// the lowest tier named.
func networkRule(cat *catalog.Catalog, line string) (models.EligibilityRule, bool) {
	ms := networkTierRe.FindAllStringSubmatch(line, -1)
	if len(ms) == 0 {
		return models.EligibilityRule{}, false
	}
	network := lookup(networkWords, ms[0][1])
	return models.EligibilityRule{Scope: models.ScopeNetwork, Network: network, MinTier: lowestTier(cat, network, line)}, true
}

// lowestTier is the lowest tier of network named right after the network's name.
func lowestTier(cat *catalog.Catalog, network, s string) string {
	best, bestIdx := "", -1
	for _, m := range networkTierRe.FindAllStringSubmatch(s, -1) {
		if lookup(networkWords, m[1]) != network {
			continue
		}
		t := lookup(tierWords, m[2])
		if i := cat.TierIndex(network, t); i >= 0 && (bestIdx < 0 || i < bestIdx) {
			best, bestIdx = t, i
		}
	}
	return best
}

func allNetworkWords() []string {
	var out []string
	for _, w := range networkWords {
		out = append(out, w...)
	}
	return out
}

func exactAlias(m *matcher, name string) string {
	n := normText(name)
	for _, e := range m.entries {
		if e.text == n {
			return e.key
		}
	}
	return ""
}

func appendNew(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

func appendRule(rules []models.EligibilityRule, r models.EligibilityRule) []models.EligibilityRule {
	for _, x := range rules {
		if x.Scope == r.Scope && x.Network == r.Network && x.MinTier == r.MinTier && x.Issuer == r.Issuer && slices.Equal(x.CardKeys, r.CardKeys) {
			return rules
		}
	}
	return append(rules, r)
}
