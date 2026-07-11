package handler

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type NemesisHandler struct {
	db     *gorm.DB
	dbOnce sync.Once
	dbErr  error
}

func NewNemesisHandler() *NemesisHandler {
	return &NemesisHandler{}
}

func (h *NemesisHandler) getDB() (*gorm.DB, error) {
	h.dbOnce.Do(func() {
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
		h.db = db
	})
	return h.db, h.dbErr
}

func (h *NemesisHandler) Query(c *gin.Context) {
	term := c.Query("q")
	if term == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	location := c.Query("location")
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	db, err := h.getDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect to Nemesis: %v", err)})
		return
	}

	type ProcurementRecord struct {
		ID                  int     `json:"id" gorm:"column:id"`
		PackageName         string  `json:"package_name" gorm:"column:package_name"`
		WorkDescription     string  `json:"work_description" gorm:"column:work_description"`
		Location            string  `json:"location" gorm:"column:location"`
		BudgetAmount        float64 `json:"budget_amount" gorm:"column:budget_amount"`
		WastePotentialScore float64 `json:"waste_potential_score" gorm:"column:waste_potential_score"`
	}

	var results []ProcurementRecord
	query := db.Table("procurement")

	likePattern := "%" + term + "%"
	if location != "" {
		locPattern := "%" + location + "%"
		query = query.Where("(package_name ILIKE ? OR work_description ILIKE ?) AND location ILIKE ?", likePattern, likePattern, locPattern)
	} else {
		query = query.Where("package_name ILIKE ? OR work_description ILIKE ?", likePattern, likePattern)
	}

	err = query.Limit(limit).Find(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Query execution failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"count":  len(results),
		"data":   results,
	})
}
