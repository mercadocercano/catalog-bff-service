package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"catalog-bff-service/src/dto"
	"catalog-bff-service/src/handler"
	"catalog-bff-service/src/infrastructure/stock/client"
)

// MockPIMServer crea un servidor mock de PIM para testing
func MockPIMServer() *httptest.Server {
	router := gin.New()

	// Mock: Listar productos
	router.GET("/api/v1/products", func(c *gin.Context) {
		c.JSON(http.StatusOK, dto.PIMProductListResponse{
			Products: []dto.PIMProductItem{
				{
					ID:         "prod-001",
					Name:       "Producto Test 1",
					CategoryID: "cat-001",
					BrandID:    "brand-001",
					Status:     "active",
				},
				{
					ID:         "prod-002",
					Name:       "Producto Test 2",
					CategoryID: "cat-002",
					BrandID:    "brand-002",
					Status:     "draft",
				},
			},
			Pagination: dto.PIMPagination{
				Page:       1,
				PageSize:   20,
				TotalItems: 2,
				TotalPages: 1,
			},
		})
	})

	// Mock: Obtener producto
	router.GET("/api/v1/products/:id", func(c *gin.Context) {
		productID := c.Param("id")
		
		if productID == "prod-001" {
			c.JSON(http.StatusOK, dto.PIMProductResponse{
				ID:          "prod-001",
				Name:        "Producto Test 1",
				Description: "Descripción del producto test",
				CategoryID:  "cat-001",
				BrandID:     "brand-001",
				Status:      "active",
			})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		}
	})

	// Mock: Crear producto
	router.POST("/api/v1/products", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		c.JSON(http.StatusCreated, dto.PIMProductResponse{
			ID:          "prod-new",
			Name:        req["name"].(string),
			Description: req["description"].(string),
			Status:      req["status"].(string),
		})
	})

	// Mock: Actualizar producto
	router.PUT("/api/v1/products/:id", func(c *gin.Context) {
		productID := c.Param("id")
		
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		if productID == "prod-001" {
			c.JSON(http.StatusOK, dto.PIMProductResponse{
				ID:          productID,
				Name:        req["name"].(string),
				Description: req["description"].(string),
				Status:      req["status"].(string),
			})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		}
	})

	// Mock: Listar variantes de un producto
	router.GET("/api/v1/products/:id/variants", func(c *gin.Context) {
		productID := c.Param("id")

		if productID == "prod-001" {
			c.JSON(http.StatusOK, gin.H{
				"variants": []dto.PIMVariantResponse{
					{
						ID:        "var-001",
						ProductID: productID,
						Name:      "Variante 1",
						SKU:       "TEST-SKU-001",
						Price:     1500.00,
						IsDefault: true,
						Status:    "active",
					},
				},
				"pagination": gin.H{
					"page": 1, "page_size": 20, "total_items": 1, "total_pages": 1,
				},
			})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		}
	})

	// Mock: Crear variante
	router.POST("/api/v1/products/:id/variants", func(c *gin.Context) {
		productID := c.Param("id")

		var req dto.CreateVariantRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		if productID == "prod-001" {
			c.JSON(http.StatusCreated, dto.PIMVariantResponse{
				ID:        "var-new",
				ProductID: productID,
				Name:      req.Name,
				SKU:       req.SKU,
				Price:     req.Price,
				IsDefault: req.IsDefault,
				Status:    "active",
			})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		}
	})

	return httptest.NewServer(router)
}

// MockStockClient implementa StockAvailabilityClient para testing
type MockStockClient struct{}

func (m *MockStockClient) GetAvailability(ctx context.Context, tenantID, sku string) (*client.StockAvailability, error) {
	return &client.StockAvailability{
		ProductSKU:        sku,
		AvailableQuantity: 100,
		ReservedQuantity:  10,
		TotalQuantity:     110,
		IsLowStock:        false,
		IsOutOfStock:      false,
	}, nil
}

// TestProductHandlerListProducts prueba el listado de productos
func TestProductHandlerListProducts(t *testing.T) {
	// Setup
	mockPIM := MockPIMServer()
	defer mockPIM.Close()

	productHandler := handler.NewProductHandler(mockPIM.URL)

	router := gin.New()
	router.GET("/api/v1/backoffice/products", productHandler.ListProducts)

	// Test
	req := httptest.NewRequest("GET", "/api/v1/backoffice/products?page=1&page_size=20", nil)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.ProductListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 2, len(response.Items))
	assert.Equal(t, 2, response.TotalCount)
	assert.Equal(t, "Producto Test 1", response.Items[0].Name)
	assert.Equal(t, "active", response.Items[0].Status)
}

// TestProductHandlerGetProduct prueba obtener un producto
func TestProductHandlerGetProduct(t *testing.T) {
	// Setup
	mockPIM := MockPIMServer()
	defer mockPIM.Close()

	productHandler := handler.NewProductHandler(mockPIM.URL)

	router := gin.New()
	router.GET("/api/v1/backoffice/products/:id", productHandler.GetProduct)

	// Test: Producto existente
	t.Run("Producto existente", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/backoffice/products/prod-001", nil)
		req.Header.Set("X-Tenant-ID", "test-tenant")
		req.Header.Set("Authorization", "Bearer test-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response dto.ProductDetailResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "prod-001", response.ID)
		assert.Equal(t, "Producto Test 1", response.Name)
		assert.Equal(t, "active", response.Status)
	})

	// Test: Producto no encontrado
	t.Run("Producto no encontrado", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/backoffice/products/prod-999", nil)
		req.Header.Set("X-Tenant-ID", "test-tenant")
		req.Header.Set("Authorization", "Bearer test-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestProductHandlerCreateProduct prueba crear un producto
func TestProductHandlerCreateProduct(t *testing.T) {
	// Setup
	mockPIM := MockPIMServer()
	defer mockPIM.Close()

	productHandler := handler.NewProductHandler(mockPIM.URL)

	router := gin.New()
	router.POST("/api/v1/backoffice/products", productHandler.CreateProduct)

	// Test: Crear producto válido
	t.Run("Crear producto válido", func(t *testing.T) {
		createReq := dto.CreateProductRequest{
			Name:        "Nuevo Producto",
			Description: "Descripción del nuevo producto",
			Status:      "active",
			Variants: []dto.CreateVariantRequest{
				{
					Name:      "Variante Default",
					SKU:       "NEW-SKU-001",
					Price:     1000.00,
					IsDefault: true,
				},
			},
		}

		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest("POST", "/api/v1/backoffice/products", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "test-tenant")
		req.Header.Set("Authorization", "Bearer test-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response dto.ProductCreatedResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "prod-new", response.ID)
		assert.Equal(t, "Nuevo Producto", response.Name)
	})

	// Test: Crear producto sin nombre
	t.Run("Crear producto sin nombre", func(t *testing.T) {
		createReq := dto.CreateProductRequest{
			Name:   "",
			Status: "active",
		}

		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest("POST", "/api/v1/backoffice/products", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "test-tenant")
		req.Header.Set("Authorization", "Bearer test-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestProductHandlerUpdateProduct prueba actualizar un producto
func TestProductHandlerUpdateProduct(t *testing.T) {
	// Setup
	mockPIM := MockPIMServer()
	defer mockPIM.Close()

	productHandler := handler.NewProductHandler(mockPIM.URL)

	router := gin.New()
	router.PUT("/api/v1/backoffice/products/:id", productHandler.UpdateProduct)

	// Test: Actualizar producto existente
	t.Run("Actualizar producto existente", func(t *testing.T) {
		updateReq := dto.UpdateProductRequest{
			Name:        "Producto Actualizado",
			Description: "Nueva descripción",
			Status:      "inactive",
		}

		body, _ := json.Marshal(updateReq)
		req := httptest.NewRequest("PUT", "/api/v1/backoffice/products/prod-001", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "test-tenant")
		req.Header.Set("Authorization", "Bearer test-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response dto.PIMProductResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Producto Actualizado", response.Name)
		assert.Equal(t, "inactive", response.Status)
	})
}

// TestVariantHandlerListVariants prueba listar variantes
func TestVariantHandlerListVariants(t *testing.T) {
	// Setup
	mockPIM := MockPIMServer()
	defer mockPIM.Close()

	mockStockClient := &MockStockClient{}
	variantHandler := handler.NewProductVariantHandler(mockPIM.URL, mockStockClient)

	router := gin.New()
	router.GET("/api/v1/backoffice/products/:id/variants", variantHandler.ListProductVariants)

	// Test
	req := httptest.NewRequest("GET", "/api/v1/backoffice/products/prod-001/variants", nil)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response []dto.VariantSummary
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 1, len(response))
	assert.Equal(t, "var-001", response[0].ID)
	assert.Equal(t, "TEST-SKU-001", response[0].SKU)
	assert.True(t, response[0].IsDefault)
	
	// Verificar que el stock fue enriquecido
	require.NotNil(t, response[0].Stock)
	assert.Equal(t, float64(100), response[0].Stock.Available)
}

// TestVariantHandlerCreateVariant prueba crear una variante
func TestVariantHandlerCreateVariant(t *testing.T) {
	// Setup
	mockPIM := MockPIMServer()
	defer mockPIM.Close()

	mockStockClient := &MockStockClient{}
	variantHandler := handler.NewProductVariantHandler(mockPIM.URL, mockStockClient)

	router := gin.New()
	router.POST("/api/v1/backoffice/products/:id/variants", variantHandler.CreateVariant)

	// Test: Crear variante válida
	t.Run("Crear variante válida", func(t *testing.T) {
		createReq := dto.CreateVariantRequest{
			Name:      "Nueva Variante",
			SKU:       "NEW-VAR-001",
			Price:     1200.00,
			IsDefault: false,
		}

		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest("POST", "/api/v1/backoffice/products/prod-001/variants", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "test-tenant")
		req.Header.Set("Authorization", "Bearer test-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response dto.VariantCreatedResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "var-new", response.ID)
		assert.Equal(t, "prod-001", response.ProductID)
		assert.Equal(t, "Nueva Variante", response.Name)
		assert.Equal(t, "NEW-VAR-001", response.SKU)
	})
}

// TestMissingTenantID prueba que se rechacen requests sin tenant ID
func TestMissingTenantID(t *testing.T) {
	mockPIM := MockPIMServer()
	defer mockPIM.Close()

	productHandler := handler.NewProductHandler(mockPIM.URL)

	router := gin.New()
	router.GET("/api/v1/backoffice/products", productHandler.ListProducts)

	// Test sin X-Tenant-ID
	req := httptest.NewRequest("GET", "/api/v1/backoffice/products", nil)
	// No se agrega X-Tenant-ID

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "missing_tenant", response.Error)
}
