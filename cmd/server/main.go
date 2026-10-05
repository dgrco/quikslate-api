package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	_ "github.com/dgrco/quikslate/docs"
	"github.com/dgrco/quikslate/internal/config"
	"github.com/dgrco/quikslate/internal/database"
	"github.com/dgrco/quikslate/internal/handler"
	"github.com/dgrco/quikslate/internal/infra/mail"
	"github.com/dgrco/quikslate/internal/infra/repo"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// @title						QuikSlate API
// @version					1.0
// @description				Scheduling API for businesses, locations, positions, shifts, and employees.
// @BasePath					/v1
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(cfg.DatabaseUrl)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	log.Println("Database connected")

	pgRepo := repo.NewPgRepository(pool)

	mailer := mail.NewLoggerMailer("test.example") // TODO: use env

	authService := service.NewAuthService(pgRepo, mailer, cfg.JWTSecret)
	authHandler := handler.NewAuthHandler(authService, cfg.JWTSecret, cfg.IsSecureMode())

	businessService := service.NewBusinessService(pgRepo)
	businessHandler := handler.NewBusinessHandler(businessService, authService, cfg.JWTSecret)

	locationService := service.NewLocationService(pgRepo)
	locationHandler := handler.NewLocationHandler(locationService, authService, cfg.JWTSecret)

	inviteService := service.NewInviteService(pgRepo)
	inviteHandler := handler.NewInviteHandler(inviteService, authService, cfg.JWTSecret)

	positionService := service.NewPositionService(pgRepo)
	positionHandler := handler.NewPositionHandler(positionService, authService, cfg.JWTSecret)

	shiftService := service.NewShiftService(pgRepo)
	shiftHandler := handler.NewShiftHandler(shiftService, authService, cfg.JWTSecret)

	employeeService := service.NewEmployeeService(pgRepo)
	employeeHandler := handler.NewEmployeeHandler(employeeService, authService, cfg.JWTSecret)

	locationRoleService := service.NewLocationRoleService(pgRepo)
	locationRoleHandler := handler.NewLocationRoleHandler(locationRoleService, authService, cfg.JWTSecret)

	userService := service.NewUserService(pgRepo)
	userHandler := handler.NewUserHandler(userService, authService, cfg.JWTSecret, cfg.IsSecureMode())

	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CorsOriginList(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Everything the API serves lives under /v1. In production a single
	// origin serves both halves of the app: the reverse proxy sends /v1/* to
	// this server and every other path to the frontend, so the prefix is what
	// keeps API routes from colliding with the app's own routes, and it lets
	// that proxy rule stay fixed as new resources are added.
	//
	// The refresh token cookie's Path is scoped to match this prefix (see
	// setRefreshTokenCookie in internal/handler/auth.go). Changing the prefix
	// here without changing it there means the browser silently stops sending
	// the cookie, which looks like every session dying on reload rather than
	// like a routing change.
	r.Route("/v1", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			response.WriteJSON(w, handler.SimpleResponse{Message: "Hello from QuikSlate!"}, http.StatusOK)
		})

		r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
			if err := pool.Ping(r.Context()); err != nil {
				response.WriteError(w, "unhealthy", http.StatusInternalServerError)
				return
			}
			response.WriteJSON(w, handler.SimpleResponse{Message: "ok"}, http.StatusOK)
		})
		r.Head("/healthz", func(w http.ResponseWriter, r *http.Request) {
			if err := pool.Ping(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		// Only outside secure mode, so prod doesn't hand out a route map for free.
		if !cfg.IsSecureMode() {
			r.Get("/swagger/*", httpSwagger.WrapHandler)
		}

		authHandler.SetupRoutes(r)
		businessHandler.SetupRoutes(r)
		locationHandler.SetupRoutes(r)
		inviteHandler.SetupRoutes(r)
		positionHandler.SetupRoutes(r)
		shiftHandler.SetupRoutes(r)
		employeeHandler.SetupRoutes(r)
		locationRoleHandler.SetupRoutes(r)
		userHandler.SetupRoutes(r)
	})

	// All four timeouts are load-bearing: without them a slow-loris client
	// holds connections open indefinitely.
	server := http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.ApiPort),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Server started on port %s", cfg.ApiPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutdown signal received.")

	// Restores default signal handling, so a second Ctrl+C force-exits
	// instead of waiting out the shutdown timeout.
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}

	log.Println("Server shutdown successfully.")
}
