// Package catalog is the list of card networks, credit cards and merchants the app
// knows. Cards and merchants live in the YAML files next to this one (reviewed in
// git); networks and their tier order are below.
package catalog

import (
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Network is a card scheme. Tiers run from lowest to highest.
type Network struct {
	Key    string
	NameTH string
	NameEN string
	Tiers  []string
}

// Card is one credit card product.
type Card struct {
	Key      string   `yaml:"key"`
	Bank     string   `yaml:"bank"`
	NameTH   string   `yaml:"name_th"`
	NameEN   string   `yaml:"name_en"`
	Aliases  []string `yaml:"aliases"` // how banks write it in promotion text
	Network  string   `yaml:"network"`
	Tier     string   `yaml:"tier"`
	CoBrand  string   `yaml:"co_brand"`
	ApplyURL string   `yaml:"apply_url"`
	ImageURL string   `yaml:"image_url"`
	Verified bool     `yaml:"verified"` // network/tier checked against the bank's page
}

// Merchant is a shop or brand promotions are tied to.
type Merchant struct {
	Key      string   `yaml:"key"`
	NameTH   string   `yaml:"name_th"`
	NameEN   string   `yaml:"name_en"`
	Category string   `yaml:"category"`
	Aliases  []string `yaml:"aliases"`
}

type Catalog struct {
	Networks  map[string]Network
	Cards     []Card
	Merchants []Merchant
}

var networks = map[string]Network{
	"visa":       {"visa", "วีซ่า", "Visa", []string{"classic", "gold", "platinum", "signature", "infinite"}},
	"mastercard": {"mastercard", "มาสเตอร์การ์ด", "Mastercard", []string{"standard", "classic", "gold", "titanium", "platinum", "world", "world-elite"}},
	"jcb":        {"jcb", "เจซีบี", "JCB", []string{"standard", "gold", "platinum", "ultimate"}},
	"unionpay":   {"unionpay", "ยูเนี่ยนเพย์", "UnionPay", []string{"standard", "gold", "platinum", "diamond"}},
	"amex":       {"amex", "อเมริกัน เอ็กซ์เพรส", "American Express", []string{"green", "gold", "platinum"}},
}

//go:embed cards.yaml
var cardsYAML []byte

//go:embed merchants.yaml
var merchantsYAML []byte

// Load reads the embedded catalog.
func Load() (*Catalog, error) { return Parse(cardsYAML, merchantsYAML) }

// Parse reads and checks a catalog.
func Parse(cards, merchants []byte) (*Catalog, error) {
	c := &Catalog{Networks: networks}
	if err := yaml.Unmarshal(cards, &c.Cards); err != nil {
		return nil, fmt.Errorf("cards.yaml: %w", err)
	}
	if err := yaml.Unmarshal(merchants, &c.Merchants); err != nil {
		return nil, fmt.Errorf("merchants.yaml: %w", err)
	}
	var errs []error
	seen := map[string]bool{}
	for _, k := range c.Cards {
		switch {
		case k.Key == "":
			errs = append(errs, errors.New("card without key"))
		case seen[k.Key]:
			errs = append(errs, fmt.Errorf("card %s: duplicate key", k.Key))
		case k.Bank == "":
			errs = append(errs, fmt.Errorf("card %s: no bank", k.Key))
		case c.Networks[k.Network].Key == "":
			errs = append(errs, fmt.Errorf("card %s: unknown network %q", k.Key, k.Network))
		case k.Tier != "" && c.TierIndex(k.Network, k.Tier) < 0:
			errs = append(errs, fmt.Errorf("card %s: tier %q not in %s", k.Key, k.Tier, k.Network))
		}
		seen[k.Key] = true
	}
	seen = map[string]bool{}
	for _, m := range c.Merchants {
		if m.Key == "" || seen[m.Key] {
			errs = append(errs, fmt.Errorf("merchant %q: missing or duplicate key", m.Key))
		}
		seen[m.Key] = true
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

// NormKey is how keys from users are compared.
func NormKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (c *Catalog) Card(key string) (Card, bool) {
	key = NormKey(key)
	for _, k := range c.Cards {
		if k.Key == key {
			return k, true
		}
	}
	return Card{}, false
}

func (c *Catalog) Merchant(key string) (Merchant, bool) {
	key = NormKey(key)
	for _, m := range c.Merchants {
		if m.Key == key {
			return m, true
		}
	}
	return Merchant{}, false
}

// ResolveMerchant finds a merchant by key, name or alias, ignoring case.
func (c *Catalog) ResolveMerchant(s string) (Merchant, bool) {
	s = NormKey(s)
	if s == "" {
		return Merchant{}, false
	}
	for _, m := range c.Merchants {
		names := append([]string{m.Key, m.NameTH, m.NameEN}, m.Aliases...)
		if slices.ContainsFunc(names, func(n string) bool { return NormKey(n) == s }) {
			return m, true
		}
	}
	return Merchant{}, false
}

func (c *Catalog) MerchantsIn(category string) []Merchant {
	var out []Merchant
	for _, m := range c.Merchants {
		if m.Category == category {
			out = append(out, m)
		}
	}
	return out
}

// TierIndex is the tier's rank within its network, -1 when unknown.
func (c *Catalog) TierIndex(network, tier string) int {
	return slices.Index(c.Networks[network].Tiers, tier)
}
