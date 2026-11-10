package normalize

import (
	"slices"
	"testing"
)

// "KTC CASH BACK" names a family: the Visa and the Mastercard card.
func TestMatcher_SameAliasSeveralCards(t *testing.T) {
	m := newMatcher(map[string][]string{
		"ktc-cash-back-visa-platinum":       {"KTC CASH BACK VISA PLATINUM", "KTC CASH BACK"},
		"ktc-cash-back-platinum-mastercard": {"KTC CASH BACK PLATINUM MASTERCARD", "KTC CASH BACK"},
	})
	got := m.keys(normText("ยกเว้นบัตร KTC CASH BACK ทุกประเภท"))
	slices.Sort(got)
	if !slices.Equal(got, []string{"ktc-cash-back-platinum-mastercard", "ktc-cash-back-visa-platinum"}) {
		t.Errorf("got %v", got)
	}
	if got := m.keys(normText("บัตร KTC CASH BACK VISA PLATINUM")); !slices.Equal(got, []string{"ktc-cash-back-visa-platinum"}) {
		t.Errorf("full name → %v", got)
	}
}
