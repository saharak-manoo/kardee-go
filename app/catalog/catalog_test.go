package catalog

import (
	"strings"
	"testing"
)

func TestLoad_EmbeddedCatalogValid(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Card("ttb-so-smart"); !ok {
		t.Error("ttb-so-smart missing")
	}
	m, ok := c.Merchant("bangchak")
	if !ok || m.Category != "fuel" {
		t.Errorf("bangchak = %+v, %v", m, ok)
	}
}

func TestTierIndex(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.TierIndex("jcb", "ultimate") <= c.TierIndex("jcb", "platinum") {
		t.Error("ultimate should rank above platinum")
	}
	if c.TierIndex("visa", "nope") != -1 || c.TierIndex("nope", "gold") != -1 {
		t.Error("unknown network or tier should be -1")
	}
}

func TestResolveMerchant(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{"บางจาก", "Bangchak", "BANGCHAK", "bangchak", " bangchak "} {
		if m, ok := c.ResolveMerchant(in); !ok || m.Key != "bangchak" {
			t.Errorf("%q → %+v %v", in, m, ok)
		}
	}
	if _, ok := c.ResolveMerchant("ไม่มีร้านนี้"); ok {
		t.Error("unknown merchant resolved")
	}
}

func TestParse_Rejects(t *testing.T) {
	merchants := "- key: a\n  name_th: a\n  category: fuel\n"
	card := func(extra string) string {
		return "- key: k1\n  bank: ktc\n  name_th: x\n  network: visa\n  tier: gold\n" + extra
	}
	cases := map[string]struct{ cards, merchants, want string }{
		"duplicate card":     {card("") + "- key: k1\n  bank: ktc\n  name_th: y\n  network: visa\n  tier: gold\n", merchants, "k1"},
		"unknown network":    {"- key: k2\n  bank: ktc\n  name_th: x\n  network: diners\n  tier: gold\n", merchants, "k2"},
		"tier not in net":    {"- key: k3\n  bank: ktc\n  name_th: x\n  network: visa\n  tier: ultimate\n", merchants, "k3"},
		"empty bank":         {"- key: k4\n  name_th: x\n  network: visa\n  tier: gold\n", merchants, "k4"},
		"duplicate merchant": {card(""), merchants + merchants, "a"},
	}
	for name, tc := range cases {
		_, err := Parse([]byte(tc.cards), []byte(tc.merchants))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want mention of %q", name, err, tc.want)
		}
	}
}
