package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

const serviceName = "clima"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": serviceName})
	})

	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
