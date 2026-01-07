package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/skelbigo/FinanceTracker/internal/config"
	"github.com/skelbigo/FinanceTracker/internal/db"
	"github.com/skelbigo/FinanceTracker/internal/migrator"
)

func main() {
	_ = godotenv.Load()

	mode := "serve"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	switch mode {
	case "migrate":
		cmd := "up"
		if len(os.Args) > 2 {
			cmd = os.Args[2]
		}

		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("invalid environment: %v", err)
		}

		dbURL := cfg.DBURL
		if dbURL == "" {
			dbCfg := db.DBConfig{
				Host:     cfg.DBHost,
				Port:     cfg.DBPort,
				User:     cfg.DBUser,
				Password: cfg.DBPassword,
				Name:     cfg.DBName,
				SSLMode:  cfg.DBSSLMode,
			}
			dbURL = db.BuildPostgresURL(dbCfg)
		}

		if err := migrator.Run("migrations", dbURL, cmd); err != nil {
			log.Fatal(err)
		}
		fmt.Println("migrations done:", cmd)

	case "serve":
		serve()

	default:
		log.Fatalf("unknown mode: %s (use: serve|migrate)", mode)
	}
}

func serve() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid environment: %v", err)
	}

	accessTTL := time.Duration(cfg.JWTAccessTTLMinutes) * time.Minute
	refreshTTL := time.Duration(cfg.RefreshTTLDays) * 24 * time.Hour

	_ = accessTTL
	_ = refreshTTL

	dbCfg := db.DBConfig{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		Name:     cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	}

	log.Printf("connecting to postgres: %s", db.MaskedDSN(dbCfg))

	pool, err := db.NewPostgresPool(ctx, dbCfg)
	if err != nil {
		log.Fatalf("db connection error: %v", err)
	}
	defer pool.Close()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/ready", func(c *gin.Context) {
		ctxPing, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctxPing); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "db": "down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "db": "up"})
	})

	addr := fmt.Sprintf(":%d", cfg.AppPort)
	log.Printf("Starting FinanceTracker API on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
