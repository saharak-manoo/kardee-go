package api

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/models"
)

var errUserRequired = errors.New("user identifier required (pass X-Line-User-ID header or line_user_id param)")

// resolveUser extracts the user from X-Line-User-ID or X-User-ID (headers or query params).
// If a LINE User ID is provided and the user does not exist yet, a new user is created.
func resolveUser(c fiber.Ctx, db *gorm.DB) (*models.User, error) {
	lineID := strings.TrimSpace(c.Get("X-Line-User-ID"))
	if lineID == "" {
		lineID = strings.TrimSpace(c.Query("line_user_id"))
	}
	if lineID != "" {
		var u models.User
		err := db.Where("line_user_id = ?", lineID).First(&u).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			u = models.User{LineUserID: lineID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
			if err := db.Create(&u).Error; err != nil {
				return nil, err
			}
			return &u, nil
		} else if err != nil {
			return nil, err
		}
		return &u, nil
	}

	userIDStr := strings.TrimSpace(c.Get("X-User-ID"))
	if userIDStr == "" {
		userIDStr = strings.TrimSpace(c.Query("user_id"))
	}
	if userIDStr != "" {
		id, err := strconv.Atoi(userIDStr)
		if err != nil || id <= 0 {
			return nil, errors.New("invalid user id")
		}
		var u models.User
		if err := db.First(&u, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fiber.NewError(fiber.StatusNotFound, "user not found")
			}
			return nil, err
		}
		return &u, nil
	}

	return nil, fiber.NewError(fiber.StatusBadRequest, errUserRequired.Error())
}

// loadUserCardKeys retrieves the user's saved card keys from the database.
func loadUserCardKeys(db *gorm.DB, userID uint) ([]string, error) {
	var userCards []models.UserCard
	if err := db.Where("user_id = ?", userID).Order("created_at ASC").Find(&userCards).Error; err != nil {
		return nil, err
	}
	keys := make([]string, len(userCards))
	for i, uc := range userCards {
		keys[i] = uc.CardKey
	}
	return keys, nil
}

type userCardsResponse struct {
	UserID     uint       `json:"user_id"`
	LineUserID string     `json:"line_user_id,omitempty"`
	CardKeys   []string   `json:"card_keys"`
	Cards      []cardView `json:"cards"`
}

func (s *Server) buildUserCardsResponse(u *models.User, keys []string, lang string) userCardsResponse {
	resp := userCardsResponse{
		UserID:     u.ID,
		LineUserID: u.LineUserID,
		CardKeys:   keys,
		Cards:      make([]cardView, 0, len(keys)),
	}
	for _, k := range keys {
		if card, ok := s.Catalog.Card(k); ok {
			resp.Cards = append(resp.Cards, viewCard(card, lang))
		}
	}
	return resp
}

func (s *Server) getUserCards(c fiber.Ctx) error {
	lang := Lang(c)
	u, err := resolveUser(c, s.DB)
	if err != nil {
		return err
	}

	keys, err := loadUserCardKeys(s.DB, u.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(s.buildUserCardsResponse(u, keys, lang))
}

type updateUserCardsBody struct {
	Cards []string `json:"cards"`
}

func (s *Server) updateUserCards(c fiber.Ctx) error {
	lang := Lang(c)
	u, err := resolveUser(c, s.DB)
	if err != nil {
		return err
	}

	var body updateUserCardsBody
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	// Normalize and deduplicate cards
	var normKeys []string
	for _, raw := range body.Cards {
		k := catalog.NormKey(raw)
		if k == "" || slices.Contains(normKeys, k) {
			continue
		}
		normKeys = append(normKeys, k)
	}

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", u.ID).Delete(&models.UserCard{}).Error; err != nil {
			return err
		}
		if len(normKeys) > 0 {
			now := time.Now()
			rows := make([]models.UserCard, len(normKeys))
			for i, k := range normKeys {
				rows[i] = models.UserCard{UserID: u.ID, CardKey: k, CreatedAt: now}
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(s.buildUserCardsResponse(u, normKeys, lang))
}

