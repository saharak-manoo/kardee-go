package api

import (
	"errors"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saharak/kardee-go/app/auth"
	"github.com/saharak/kardee-go/app/models"
)

const sessionTTL = 30 * 24 * time.Hour

var channels = []string{"line", "web", "app"}

// channel is the client kind from X-Kardee-Channel ("web" when missing or unknown).
func channel(c fiber.Ctx) string {
	if ch := strings.ToLower(c.Get("X-Kardee-Channel")); slices.Contains(channels, ch) {
		return ch
	}
	return "web"
}

var errNoSession = fiber.NewError(fiber.StatusUnauthorized, "sign in required")

// currentUser is the user of the request's Bearer session, or nil when there is no
// Authorization header. A header that doesn't match a live session is an error.
func (s *Server) currentUser(c fiber.Ctx) (*models.User, error) {
	h := c.Get(fiber.HeaderAuthorization)
	if h == "" {
		return nil, nil
	}
	token, ok := strings.CutPrefix(h, "Bearer ")
	if !ok || token == "" {
		return nil, errNoSession
	}
	var sess models.Session
	err := s.DB.Preload("User").Where("token_hash = ? AND expires_at > ?", auth.HashToken(token), time.Now()).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || err == nil && sess.User == nil {
		return nil, errNoSession // unknown, expired, or the account was deleted
	}
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	s.DB.Model(&sess).Update("last_used_at", time.Now())
	return sess.User, nil
}

func (s *Server) requireUser(c fiber.Ctx) (*models.User, error) {
	u, err := s.currentUser(c)
	if err == nil && u == nil {
		err = errNoSession
	}
	return u, err
}

type userView struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Locale      string    `json:"locale"`
}

func viewUser(u *models.User) userView {
	return userView{u.ID, u.DisplayName, u.Locale}
}

// loginLINE trades a LIFF ID token for a Kardee session.
func (s *Server) loginLINE(c fiber.Ctx) error {
	if s.Verifier == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "LINE login is not configured")
	}
	var body struct {
		IDToken string `json:"id_token"`
	}
	if err := c.Bind().Body(&body); err != nil || body.IDToken == "" {
		return fiber.NewError(fiber.StatusBadRequest, "id_token is required")
	}
	id, err := s.Verifier.Verify(c.Context(), body.IDToken)
	if errors.Is(err, auth.ErrInvalidToken) {
		return fiber.NewError(fiber.StatusUnauthorized, "LINE did not accept the token")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, err.Error())
	}

	token, hash, err := auth.NewToken()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	var user models.User
	expires := time.Now().Add(sessionTTL)
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		var ident models.UserIdentity
		err := tx.Where("provider = ? AND subject = ?", id.Provider, id.Subject).First(&ident).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			user = models.User{ID: uuid.New(), DisplayName: id.Name, Locale: Lang(c)}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			ident = models.UserIdentity{UserID: user.ID, Provider: id.Provider, Subject: id.Subject}
			if err := tx.Create(&ident).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if err := tx.First(&user, "id = ?", ident.UserID).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&ident).Update("last_login_at", time.Now()).Error; err != nil {
			return err
		}
		return tx.Create(&models.Session{UserID: user.ID, TokenHash: hash, Channel: channel(c), ExpiresAt: expires}).Error
	})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"token": token, "expires_at": expires, "user": viewUser(&user)})
}

// deleteMe removes the account: cards, sessions and identities go, the user row is
// soft-deleted. Signing in again starts a new account.
func (s *Server) deleteMe(c fiber.Ctx) error {
	u, err := s.requireUser(c)
	if err != nil {
		return err
	}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		for _, m := range []any{&models.UserCard{}, &models.Session{}, &models.UserIdentity{}} {
			if err := tx.Where("user_id = ?", u.ID).Delete(m).Error; err != nil {
				return err
			}
		}
		return tx.Delete(u).Error
	})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// savedCards is the user's wallet in their order.
func (s *Server) savedCards(userID uuid.UUID) ([]string, error) {
	var rows []models.UserCard
	if err := s.DB.Where("user_id = ?", userID).Order("position").Find(&rows).Error; err != nil {
		return nil, err
	}
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = r.CardKey
	}
	return keys, nil
}

func (s *Server) cardsResponse(c fiber.Ctx, keys []string) error {
	lang := Lang(c)
	out := make([]cardView, 0, len(keys))
	for _, k := range keys {
		if card, ok := s.Catalog.Card(k); ok {
			out = append(out, viewCard(card, lang))
		}
	}
	return c.JSON(fiber.Map{"lang": lang, "cards": out})
}

func (s *Server) getMyCards(c fiber.Ctx) error {
	u, err := s.requireUser(c)
	if err != nil {
		return err
	}
	keys, err := s.savedCards(u.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return s.cardsResponse(c, keys)
}

// putMyCards replaces the wallet; the body's order is the user's order.
func (s *Server) putMyCards(c fiber.Ctx) error {
	u, err := s.requireUser(c)
	if err != nil {
		return err
	}
	var body struct {
		Cards []string `json:"cards"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	keys := parseCards(strings.Join(body.Cards, ","))
	var unknown []string
	for _, k := range keys {
		if _, ok := s.Catalog.Card(k); !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		return fiber.NewError(fiber.StatusBadRequest, "unknown cards: "+strings.Join(unknown, ", "))
	}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", u.ID).Delete(&models.UserCard{}).Error; err != nil {
			return err
		}
		if len(keys) == 0 {
			return nil
		}
		now := time.Now()
		rows := make([]models.UserCard, len(keys))
		for i, k := range keys {
			rows[i] = models.UserCard{UserID: u.ID, CardKey: k, Position: i, AddedAt: now}
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return s.cardsResponse(c, keys)
}

// goApply records an "apply" tap and sends the user to the card's apply page. It only
// redirects to URLs from the catalog, never to one given in the request.
func (s *Server) goApply(c fiber.Ctx) error {
	card, ok := s.Catalog.Card(c.Query("card"))
	if !ok || card.ApplyURL == "" {
		return fiber.NewError(fiber.StatusNotFound, "unknown card")
	}
	click := models.AffiliateClick{CardKey: card.Key, Channel: channel(c)}
	if id, err := strconv.ParseUint(c.Query("promotion"), 10, 64); err == nil && id > 0 {
		pid := uint(id)
		click.PromotionID = &pid
	}
	if u, err := s.currentUser(c); err == nil && u != nil {
		click.UserID = &u.ID
	}
	if err := s.DB.Create(&click).Error; err != nil {
		log.Printf("[api] affiliate click: %v", err) // never block the user's redirect
	}
	return c.Redirect().Status(fiber.StatusFound).To(card.ApplyURL)
}

// logSearch records a recommend request; failures are logged, not returned.
func (s *Server) logSearch(c fiber.Ctx, u *models.User, merchant string, amount, cards, results int) {
	l := models.SearchLog{Channel: channel(c), MerchantKey: merchant, Amount: amount, CardsCount: cards, ResultsCount: results}
	if u != nil {
		l.UserID = &u.ID
	}
	if err := s.DB.Create(&l).Error; err != nil {
		log.Printf("[api] search log: %v", err)
	}
}
