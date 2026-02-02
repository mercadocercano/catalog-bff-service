package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	
	"catalog-bff-service/src/handler"
)

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func main() {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "service": "catalog-bff-service"})
	})

	// API v1
	v1 := router.Group("/api/v1")
	{
		catalog := v1.Group("/catalog")
		{
			// HITO 1: Endpoint agregado variante + stock
			catalog.GET("/variants/:id", handler.GetVariantWithStock)
		}
	}

	port := getEnv("PORT", "8085")
	log.Printf("Catalog BFF service iniciando en http://localhost:%s", port)
	router.Run(":" + port)
}
