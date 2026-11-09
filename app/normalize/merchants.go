package normalize

import (
	"regexp"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/models"
)

// MerchantMatch is a catalog merchant a promotion applies at.
type MerchantMatch struct {
	Key   string
	Match string // models.MatchMerchant or models.MatchCategory
}

var (
	fuelWordRe    = regexp.MustCompile(`น้ำมัน|FUEL|PETROL|GASOLINE`)
	fuelStationRe = regexp.MustCompile(`ปั๊ม|สถานีบริการ|STATION`)
)

// ExtractMerchants finds the merchants text names. A fuel promotion that names no
// station ("ที่ปั๊มที่ร่วมรายการ") applies at every fuel merchant, marked as a category match.
func ExtractMerchants(cat *catalog.Catalog, text string) []MerchantMatch {
	aliases := map[string][]string{}
	for _, m := range cat.Merchants {
		aliases[m.Key] = append([]string{m.NameTH, m.NameEN}, m.Aliases...)
	}
	norm := normText(text)
	var out []MerchantMatch
	for _, k := range newMatcher(aliases).keys(norm) {
		out = append(out, MerchantMatch{k, models.MatchMerchant})
	}
	if len(out) == 0 && fuelWordRe.MatchString(norm) && fuelStationRe.MatchString(norm) {
		for _, m := range cat.MerchantsIn("fuel") {
			out = append(out, MerchantMatch{m.Key, models.MatchCategory})
		}
	}
	return out
}
