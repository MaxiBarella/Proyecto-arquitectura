package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/adapters/mysql"
)

const serviceName = "vuelos"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("falta la variable MYSQL_DSN")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := mysql.Open(ctx, dsn)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	// Lo usan los comandos y los consumidores de eventos, que vienen en otro PR.
	_ = mysql.NewRepository(db)

	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": serviceName})
	})

	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
