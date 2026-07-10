package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dgrco/quikslate/internal/config"
	"github.com/dgrco/quikslate/internal/database"
	"github.com/dgrco/quikslate/internal/handler"
	"github.com/dgrco/quikslate/internal/infra/repo"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Load environment
	cfg := config.Load()

	// Connect to database
	pool, err := database.Connect(cfg.DatabaseUrl)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	log.Println("Database connected")

	pgRepo := repo.NewPgRepository(pool)

	// Auth service/handler
	authService := service.NewAuthService(pgRepo, cfg.JWTSecret)
	authHandler := handler.NewAuthHandler(authService, cfg.IsSecureMode(), cfg.JWTSecret)

	// Other service/handlers
	businessService := service.NewBusinessService(pgRepo)
	businessHandler := handler.NewBusinessHandler(businessService, cfg.JWTSecret)

	locationService := service.NewLocationService(pgRepo)
	locationHandler := handler.NewLocationHandler(locationService, cfg.JWTSecret)

	inviteService := service.NewInviteService(pgRepo)
	inviteHandler := handler.NewInviteHandler(inviteService, authService, cfg.JWTSecret)

	// Setup router
	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)

	// Route Setup
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi from Quikslate :)"))
	})

	authHandler.SetupRoutes(r)
	businessHandler.SetupRoutes(r)
	locationHandler.SetupRoutes(r)
	inviteHandler.SetupRoutes(r)

	// Listen
	log.Printf("Server started on port %s", cfg.ApiPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", cfg.ApiPort), r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
