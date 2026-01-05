package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/internal/db"
)

func main() {
	ctx := context.Background()

	pool, err := db.NewPostgresPool(ctx)
	if err != nil {
		log.Fatalf("db connection error: %v", err)
	}
	defer pool.Close()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/ready", func(c *gin.Context) {
		if err := pool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "db": "down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "db": "up"})
	})

	log.Println("Starting FinanceTracker API on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
