package router

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/octobees/leads-generator/api/internal/auth"
	"github.com/octobees/leads-generator/api/internal/config"
	"github.com/octobees/leads-generator/api/internal/handler"
	middlewarepkg "github.com/octobees/leads-generator/api/internal/middleware"
)

// Handlers aggregates HTTP handlers used by the router.
type Handlers struct {
	Auth        *handler.AuthHandler
	Users       *handler.UserAdminHandler
	Companies   *handler.CompaniesHandler
	AdminUpload *handler.AdminUploadHandler
	Scrape      *handler.ScrapeHandler
	Enrich      *handler.EnrichHandler
	EnrichJob   *handler.EnrichWorkerHandler
	Prompt      *handler.PromptSearchHandler
}

// Register wires all HTTP routes for the API.
func Register(e *echo.Echo, cfg *config.Config, jwtManager *auth.JWTManager, handlers Handlers) {
	e.GET("/healthz", func(c echo.Context) error {
		return handler.Success(c, http.StatusOK, "service healthy", map[string]any{"status": "ok"})
	})

	e.POST("/auth/register", handlers.Auth.Register)
	e.POST("/auth/login", handlers.Auth.Login)
	e.GET("/companies", handlers.Companies.List)

	if handlers.Enrich != nil {
		e.POST("/enrich-result", handlers.Enrich.SaveResult)
		e.GET("/enrich-result/:company_id", handlers.Enrich.GetResult)
	}

	secured := e.Group("")
	secured.Use(middlewarepkg.JWT(jwtManager))

	admin := secured.Group("/admin", middlewarepkg.RequireRole("admin"))
	admin.GET("/companies", handlers.Companies.ListAdmin)
	admin.POST("/upload-csv", handlers.AdminUpload.UploadCSV)
	admin.GET("/users", handlers.Users.List)
	admin.POST("/users", handlers.Users.Create)
	admin.PATCH("/users/:id", handlers.Users.Update)
	admin.DELETE("/users/:id", handlers.Users.Delete)

	secured.POST("/scrape", handlers.Scrape.Enqueue, middlewarepkg.ScrapeRateLimiter(cfg.RateLimitScrape))
	if handlers.EnrichJob != nil {
		secured.POST("/enrich", handlers.EnrichJob.Enqueue, middlewarepkg.ScrapeRateLimiter(cfg.RateLimitScrape))
	}
	if handlers.Prompt != nil {
		secured.POST("/prompt-search", handlers.Prompt.Enqueue, middlewarepkg.ScrapeRateLimiter(cfg.RateLimitScrape))
	}
}
