package router

import (
	"github.com/gin-gonic/gin"

	batchhandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/batch"
	categoryhandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/category"
	docshandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/docs"
	healthhandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/health"
	producthandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/product"
	"github.com/leyl1ne/ProductService/internal/transport/http/middleware"
	"github.com/leyl1ne/ProductService/pkg/httpserver"
	"github.com/leyl1ne/ProductService/pkg/logger"
)

// Deps holds all collaborators required to build the HTTP router.
// Wire it up at composition root (cmd/.../main.go) and pass to New.
type Handlers struct {
	ProductHandler  *producthandler.Handler
	BatchHandler    *batchhandler.Handler
	CategoryHandler *categoryhandler.Handler
	DocsHandler     *docshandler.DocsHandler
	HealthHandler   *healthhandler.HealthHandler
}

func SetupRouter(
	log logger.Logger,
	handlers Handlers,
	cfg httpserver.Config,
) *gin.Engine {
	router := gin.New()

	router.Use(
		gin.Recovery(),
		middleware.LoggerMiddleware(log),
		middleware.ServiceBasicAuthMiddleware(log, cfg.HTTP.BasicAuth),
	)

	// Health check
	router.GET("/health", handlers.HealthHandler.Health())

	// Documentation for api
	router.GET("/docs", handlers.DocsHandler.Redirect())
	router.GET("/swagger", handlers.DocsHandler.UI())
	router.GET("/swagger/api.yaml", handlers.DocsHandler.Spec())

	authorized := router.Group("")
	authorized.Use(middleware.ExtractHeadersMiddleware(log))
	{
		// ---- Products -------------------------------------------------------
		authorized.GET("/products", handlers.ProductHandler.ListProducts())
		authorized.POST("/products", handlers.ProductHandler.CreateProduct())

		authorized.GET("/products/:id", handlers.ProductHandler.GetProduct())
		authorized.PATCH("/products/:id", handlers.ProductHandler.UpdateProduct())
		authorized.DELETE("/products/:id", handlers.ProductHandler.DeleteProduct())

		authorized.GET("/products/:id/availability", handlers.ProductHandler.GetProductAvailability())

		// ---- Batches (nested under product) ---------------------------------
		authorized.GET("/products/:id/batches", handlers.BatchHandler.ListBatchesByProduct())
		authorized.POST("/products/:id/batches", handlers.BatchHandler.CreateBatch())

		// ---- Batches (standalone) -------------------------------------------
		authorized.GET("/batches/:id", handlers.BatchHandler.GetBatch())
		authorized.PATCH("/batches/:id", handlers.BatchHandler.UpdateBatch())
		authorized.DELETE("/batches/:id", handlers.BatchHandler.DeleteBatch())

		// ---- Categories -----------------------------------------------------
		authorized.GET("/categories", handlers.CategoryHandler.ListCategories())
		authorized.POST("/categories", handlers.CategoryHandler.CreateCategory())

		authorized.PATCH("/categories/:id", handlers.CategoryHandler.UpdateCategory())
		authorized.DELETE("/categories/:id", handlers.CategoryHandler.DeleteCategory())
	}

	return router
}
