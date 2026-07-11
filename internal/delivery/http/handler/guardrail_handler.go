package handler

import (
	"errors"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/cache"
)

type GuardrailHandler struct {
	redis cache.Cache
	db    *gorm.DB
	once  sync.Once
	dbErr error
}

func NewGuardrailHandler(redis cache.Cache) *GuardrailHandler {
	return &GuardrailHandler{
		redis: redis,
	}
}

func (h *GuardrailHandler) getDB() (*gorm.DB, error) {
	h.once.Do(func() {
		host := os.Getenv("NEMESIS_DB_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("NEMESIS_DB_PORT")
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("NEMESIS_DB_USER")
		if user == "" {
			user = "postgres"
		}
		password := os.Getenv("NEMESIS_DB_PASSWORD")
		if password == "" {
			password = "postgres"
		}
		name := os.Getenv("NEMESIS_DB_NAME")
		if name == "" {
			name = "nemesis_db"
		}
		sslmode := os.Getenv("NEMESIS_DB_SSLMODE")
		if sslmode == "" {
			sslmode = "disable"
		}

		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, name, sslmode)
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
			SkipDefaultTransaction: true,
		})
		if err != nil {
			h.dbErr = err
			return
		}
		// Try to enable pg_trgm extension on nemesis_db
		_ = db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error
		h.db = db
	})
	return h.db, h.dbErr
}

type PrecheckItem struct {
	ItemName string  `json:"item_name" binding:"required"`
	Price    float64 `json:"price" binding:"required"`
}

type PrecheckRequest struct {
	Items []PrecheckItem `json:"items" binding:"required"`
}

type StandardPriceInfo struct {
	ItemName   string  `json:"item_name"`
	Category   string  `json:"category"`
	MaxPrice   float64 `json:"max_price"`
	MinSpecs   string  `json:"min_specs"`
	Similarity float64 `json:"similarity"`
}

type PrecheckResponseItem struct {
	ItemName         string  `json:"item_name"`
	Price            float64 `json:"price"`
	MaxPrice         float64 `json:"max_price"`
	IsViolation      bool    `json:"is_violation"`
	ExcessAmount     float64 `json:"excess_amount"`
	StandardCategory string  `json:"standard_category"`
	MinSpecs         string  `json:"min_specs"`
	MatchedName      string  `json:"matched_name,omitempty"`
	Similarity       float64 `json:"similarity,omitempty"`
}

func (h *GuardrailHandler) Precheck(c *gin.Context) {
	var req PrecheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	db, err := h.getDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Nemesis DB connection error: %v", err)})
		return
	}

	redisCache, hasRedis := h.redis.(*cache.RedisCache)
	ctx := c.Request.Context()

	var responseItems []PrecheckResponseItem

	for _, item := range req.Items {
		cleanName := strings.TrimSpace(strings.ToLower(item.ItemName))
		if cleanName == "" {
			continue
		}

		var stdInfo StandardPriceInfo
		cacheKey := "guardrails:standard:" + cleanName
		cacheHit := false

		if hasRedis {
			if cachedVal, err := redisCache.GetClient().Get(ctx, cacheKey).Result(); err == nil && cachedVal != "" {
				if err := json.Unmarshal([]byte(cachedVal), &stdInfo); err == nil {
					cacheHit = true
				}
			}
		}

		if !cacheHit {
			type DBStandardPrice struct {
				ItemName     string  `gorm:"column:item_name"`
				ItemCategory string  `gorm:"column:item_category"`
				MaxPrice     float64 `gorm:"column:max_price"`
				MinSpecs     string  `gorm:"column:min_specs"`
				Similarity   float64 `gorm:"column:smpl"`
			}
			var dbItem DBStandardPrice

			// 1. Try fuzzy matching using pg_trgm first
			trgmErr := db.Raw(`
				SELECT item_name, item_category, max_price, min_specs, similarity(item_name, ?) as smpl 
				FROM standard_price 
				WHERE (item_name % ? OR similarity(item_name, ?) > 0.35)
				ORDER BY smpl DESC LIMIT 1`, item.ItemName, item.ItemName, item.ItemName).Scan(&dbItem).Error

			// 2. Fallback to standard ILIKE search if trigram returns empty or errors out
			if trgmErr != nil || dbItem.ItemName == "" {
				err = db.Table("standard_price").Where("item_name ILIKE ?", item.ItemName).First(&dbItem).Error
				if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
					words := strings.Fields(item.ItemName)
					var cleanWords []string
					for _, w := range words {
						wLower := strings.ToLower(w)
						isNum := true
						for _, ch := range w {
							if ch < '0' || ch > '9' {
								isNum = false
								break
							}
						}
						isUnit := wLower == "unit" || wLower == "sak" || wLower == "pcs" || wLower == "buah" || wLower == "box" || wLower == "kg"
						if !isNum && !isUnit {
							cleanWords = append(cleanWords, w)
						}
					}
					if len(cleanWords) > 0 {
						_ = db.Table("standard_price").Where("item_name ILIKE ?", "%"+cleanWords[0]+"%").First(&dbItem).Error
					}
				}
				if dbItem.ItemName != "" {
					dbItem.Similarity = 1.0 // ILIKE exact or substring match fallback
				}
			}

			if dbItem.ItemName != "" {
				stdInfo = StandardPriceInfo{
					ItemName:   dbItem.ItemName,
					Category:   dbItem.ItemCategory,
					MaxPrice:   dbItem.MaxPrice,
					MinSpecs:   dbItem.MinSpecs,
					Similarity: dbItem.Similarity,
				}
				if hasRedis {
					if infoBytes, err := json.Marshal(stdInfo); err == nil {
						_ = redisCache.GetClient().Set(ctx, cacheKey, infoBytes, 1*time.Hour).Err()
					}
				}
			}
		}

		isViolation := false
		excessAmount := 0.0
		if stdInfo.MaxPrice > 0 && item.Price > stdInfo.MaxPrice {
			isViolation = true
			excessAmount = item.Price - stdInfo.MaxPrice
		}

		responseItems = append(responseItems, PrecheckResponseItem{
			ItemName:         item.ItemName,
			Price:            item.Price,
			MaxPrice:         stdInfo.MaxPrice,
			IsViolation:      isViolation,
			ExcessAmount:     excessAmount,
			StandardCategory: stdInfo.Category,
			MinSpecs:         stdInfo.MinSpecs,
			MatchedName:      stdInfo.ItemName,
			Similarity:       stdInfo.Similarity,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   responseItems,
	})
}
