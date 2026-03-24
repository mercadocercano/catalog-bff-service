package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	tenantmw "github.com/mercadocercano/middleware"

	"catalog-bff-service/src/admin"
	"catalog-bff-service/src/domain"
	"catalog-bff-service/src/handler"
	"catalog-bff-service/src/infrastructure/cache"
	"catalog-bff-service/src/infrastructure/stock/client"
	tenantclient "catalog-bff-service/src/infrastructure/tenant/client"
	tenant_dashboard "catalog-bff-service/src/tenant_dashboard"
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
	router.Use(tenantmw.TenantValidation(tenantmw.TenantValidationConfig{
		JWTSecret: os.Getenv("JWT_SECRET"),
		ExcludedRoutes: []string{
			"/health",
			"/api/v1/health",
			"/metrics",
		},
	}))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "service": "catalog-bff-service"})
	})

	// Configuración de servicios
	pimServiceURL := getEnv("PIM_SERVICE_URL", "http://localhost:8090")
	stockServiceURL := getEnv("STOCK_SERVICE_URL", "http://localhost:8100")
	tenantServiceURL := getEnv("TENANT_SERVICE_URL", "")
	scraperServiceURL := getEnv("SCRAPER_SERVICE_URL", "http://localhost:8086")
	iamServiceURL := getEnv("IAM_SERVICE_URL", "http://localhost:8080")

	// Configuración de cache (TTLs configurables por env)
	tenantConfigTTL := parseDuration(getEnv("TENANT_CONFIG_CACHE_TTL", "60s"), 60*time.Second)
	stockAvailabilityTTL := parseDuration(getEnv("STOCK_CACHE_TTL", "5s"), 5*time.Second)
	cacheCleanupInterval := parseDuration(getEnv("CACHE_CLEANUP_INTERVAL", "60s"), 60*time.Second)

	log.Printf("Cache configurado: tenant_config_ttl=%s, stock_ttl=%s, cleanup=%s",
		tenantConfigTTL, stockAvailabilityTTL, cacheCleanupInterval)

	// Crear caches in-memory
	tenantConfigCache := cache.NewInMemoryCache[string](tenantConfigTTL, cacheCleanupInterval)
	stockCache := cache.NewInMemoryCache[*client.StockAvailability](stockAvailabilityTTL, cacheCleanupInterval)

	// Inicializar Tenant Config Client con cache
	var policyResolver *domain.TenantStockPolicyResolver
	if tenantServiceURL != "" {
		log.Printf("Tenant Service configurado en: %s", tenantServiceURL)
		baseTenantClient := tenantclient.NewHTTPTenantConfigClient(tenantServiceURL)
		cachedTenantClient := tenantclient.NewCachedTenantConfigClient(baseTenantClient, tenantConfigCache)
		policyResolver = domain.NewTenantStockPolicyResolver(cachedTenantClient)
	} else {
		log.Println("⚠️  TENANT_SERVICE_URL no configurado. Usando fallback: REQUIRE_STOCK")
		// Crear resolver sin client (usará fallback)
		policyResolver = domain.NewTenantStockPolicyResolver(nil)
	}

	// Inicializar Stock Client con cache
	baseStockClient := client.NewHTTPStockAvailabilityClient(stockServiceURL)
	cachedStockClient := client.NewCachedStockAvailabilityClient(baseStockClient, stockCache)

	// Inicializar handlers con dependencias
	sellableVariantsHandler := handler.NewSellableVariantsHandler(
		policyResolver,
		pimServiceURL,
		cachedStockClient,
	)

	// Handlers para backoffice CRUD
	productHandler := handler.NewProductHandler(pimServiceURL)
	variantHandler := handler.NewProductVariantHandler(pimServiceURL, cachedStockClient)

	// Handler de inventario (orquesta Stock + PIM)
	inventoryHandler := handler.NewInventoryHandler(stockServiceURL, pimServiceURL)

	// Handler para admin dashboard
	dashboardService := admin.NewDashboardService(pimServiceURL, scraperServiceURL, iamServiceURL, tenantServiceURL)
	adminHandler := admin.NewHandler(dashboardService)

	// Handler para tenant dashboard (orquesta PIM + Stock con scope de tenant)
	tenantDashboardService := tenant_dashboard.NewService(pimServiceURL, stockServiceURL)
	tenantDashboardHandler := tenant_dashboard.NewHandler(tenantDashboardService)

	// API v1
	v1 := router.Group("/api/v1")
	{
		// Endpoints de catálogo (lectura agregada)
		catalog := v1.Group("/catalog")
		{
			// HITO 1: Endpoint agregado variante + stock
			catalog.GET("/variants/:id", handler.GetVariantWithStock)
			
			// Endpoint catálogo vendible (con Stock Policy por tenant)
			catalog.GET("/sellable-variants", sellableVariantsHandler.Handle)
		}

		// Endpoints de backoffice (CRUD de productos y variantes)
		backoffice := v1.Group("/backoffice")
		{
			// Productos
			backoffice.GET("/products", productHandler.ListProducts)
			backoffice.POST("/products", productHandler.CreateProduct)
			backoffice.GET("/products/:id", productHandler.GetProduct)
			backoffice.PUT("/products/:id", productHandler.UpdateProduct)

			// Variantes (usar :id en lugar de :product_id para consistencia)
			backoffice.GET("/products/:id/variants", variantHandler.ListProductVariants)
			backoffice.POST("/products/:id/variants", variantHandler.CreateVariant)
			backoffice.GET("/products/:id/variants/:variant_id", variantHandler.GetVariant)
			backoffice.PUT("/products/:id/variants/:variant_id", variantHandler.UpdateVariant)
			backoffice.PATCH("/products/:id/variants/:variant_id/status", variantHandler.ToggleVariantStatus)
		}

		// Endpoints de inventario (Stock + PIM orquestado)
		inventory := v1.Group("/inventory")
		{
			inventory.GET("", inventoryHandler.ListInventory)
			inventory.GET("/summary", inventoryHandler.GetInventorySummary)
		}

		// Endpoints de administración (dashboard, métricas)
		adminGroup := v1.Group("/admin")
		{
			adminGroup.GET("/dashboard/stats", adminHandler.GetDashboardStats)
		}

		// Endpoints de tenant dashboard (PIM + Stock orquestado por tenant)
		tenantGroup := v1.Group("/tenant")
		{
			tenantGroup.GET("/dashboard", tenantDashboardHandler.GetDashboard)
		}
	}
	
	log.Println("Rutas disponibles:")
	log.Println("")
	log.Println("📦 Catálogo (Lectura Agregada):")
	log.Println("  GET /api/v1/catalog/variants/:id")
	log.Println("  GET /api/v1/catalog/sellable-variants")
	log.Println("")
	log.Println("🏪 Backoffice (CRUD Productos y Variantes):")
	log.Println("  GET    /api/v1/backoffice/products")
	log.Println("  GET    /api/v1/backoffice/products/:id")
	log.Println("  POST   /api/v1/backoffice/products")
	log.Println("  PUT    /api/v1/backoffice/products/:id")
	log.Println("  GET    /api/v1/backoffice/products/:id/variants")
	log.Println("  GET    /api/v1/backoffice/products/:id/variants/:variant_id")
	log.Println("  POST   /api/v1/backoffice/products/:id/variants")
	log.Println("  PUT    /api/v1/backoffice/products/:id/variants/:variant_id")
	log.Println("  PATCH  /api/v1/backoffice/products/:id/variants/:variant_id/status")
	log.Println("")
	log.Println("📦 Inventario (Stock + PIM Orquestado):")
	log.Println("  GET    /api/v1/inventory")
	log.Println("  GET    /api/v1/inventory/summary")
	log.Println("")
	log.Println("📊 Admin (Dashboard y Métricas):")
	log.Println("  GET    /api/v1/admin/dashboard/stats")
	log.Println("")
	log.Println("🏠 Tenant Dashboard (PIM + Stock por tenant):")
	log.Println("  GET    /api/v1/tenant/dashboard")
	log.Println("")
	log.Println("Configuración:")
	log.Printf("  PIM_SERVICE_URL: %s", pimServiceURL)
	log.Printf("  STOCK_SERVICE_URL: %s", stockServiceURL)
	log.Printf("  TENANT_SERVICE_URL: %s", tenantServiceURL)
	log.Printf("  SCRAPER_SERVICE_URL: %s", scraperServiceURL)
	log.Printf("  IAM_SERVICE_URL: %s", iamServiceURL)
	log.Printf("  Cache: tenant_config=%s, stock=%s", tenantConfigTTL, stockAvailabilityTTL)

	port := getEnv("PORT", "8085")
	log.Printf("Catalog BFF service iniciando en http://localhost:%s", port)
	router.Run(":" + port)
}

// parseDuration parsea una duración desde string, con fallback
func parseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("⚠️  Error parseando duración '%s': %v. Usando fallback: %s", s, err, fallback)
		return fallback
	}
	return d
}
