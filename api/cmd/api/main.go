package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"

	"github.com/octobees/leads-generator/api/internal/auth"
	"github.com/octobees/leads-generator/api/internal/config"
	"github.com/octobees/leads-generator/api/internal/database"
	"github.com/octobees/leads-generator/api/internal/handler"
	middlewarepkg "github.com/octobees/leads-generator/api/internal/middleware"
	"github.com/octobees/leads-generator/api/internal/repository"
	"github.com/octobees/leads-generator/api/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer pool.Close()

	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.TokenTTL)

	usersRepo := repository.NewPGXUsersRepository(pool)
	companiesRepo := repository.NewPGXCompaniesRepository(pool)

	authService := service.NewAuthService(usersRepo, jwtManager)
	companiesService := service.NewCompaniesService(companiesRepo)

	authHandler := handler.NewAuthHandler(authService)
	companiesHandler := handler.NewCompaniesHandler(companiesService)
	adminUploadHandler := handler.NewAdminUploadHandler(companiesService)
	httpClient := &http.Client{Timeout: 15 * time.Second}
	scrapeHandler := handler.NewScrapeHandler(httpClient, cfg.WorkerBaseURL)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middlewarepkg.RequestID())
	e.Use(middlewarepkg.Logging())
	e.Use(echoMiddleware.Recover())

	e.GET("/healthz", func(c echo.Context) error {
		return handler.Success(c, http.StatusOK, "service healthy", map[string]any{"status": "ok"})
	})

	e.POST("/auth/login", authHandler.Login)
	e.GET("/companies", companiesHandler.List)

	secured := e.Group("")
	secured.Use(middlewarepkg.JWT(jwtManager))

	admin := secured.Group("/admin", middlewarepkg.RequireRole("admin"))
	admin.GET("/companies", companiesHandler.ListAdmin)
	admin.POST("/upload-csv", adminUploadHandler.UploadCSV)

	secured.POST("/scrape", scrapeHandler.Enqueue, middlewarepkg.ScrapeRateLimiter(cfg.RateLimitScrape))

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- e.Start(":" + cfg.Port)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("received signal %s, shutting down", sig)
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
		return
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
