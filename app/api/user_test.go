package api

import (
	"testing"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/models"
)

func TestBuildUserCardsResponse(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	srv := &Server{Catalog: cat}
	user := &models.User{
		ID:         42,
		LineUserID: "U1234567890",
	}

	keys := []string{"ttb-so-smart", "ktc-visa-platinum", "unknown-card-key"}
	resp := srv.buildUserCardsResponse(user, keys, "th")

	if resp.UserID != 42 || resp.LineUserID != "U1234567890" {
		t.Errorf("unexpected user info: %+v", resp)
	}
	if len(resp.CardKeys) != 3 {
		t.Errorf("expected 3 card keys, got %d", len(resp.CardKeys))
	}
	// "unknown-card-key" is not in catalog, so Cards slice should only have the 2 known cards
	if len(resp.Cards) != 2 {
		t.Errorf("expected 2 known cards, got %d", len(resp.Cards))
	}
	if resp.Cards[0].Key != "ttb-so-smart" || resp.Cards[1].Key != "ktc-visa-platinum" {
		t.Errorf("unexpected cards: %+v", resp.Cards)
	}
}

