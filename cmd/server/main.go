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
	"github.com/dgrco/quikslate/internal/infra/repo"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// main.go is the composition root: it loads config, connects to the
// database, wires a PgRepository into each service and each service into
// its handler, and starts the chi router.

// @title						QuikSlate API
// @version					1.0
// @description				Scheduling API for businesses, locations, positions, shifts, and employees.
// @BasePath					/v1
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
func main() {
	// Load environment
	cfg := config.Load()

	// Set up SIGINT/SIGTERM catching context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	// Setup router
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

	// Route Setup
	//
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
			response.WriteJSON(w, handler.SimpleResponse{Message: "Hi from Quikslate :)"}, http.StatusOK)
		})

		// Health endpoint(s)
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


		// Swagger UI + spec, served at /v1/swagger/index.html
		r.Get("/swagger/*", httpSwagger.WrapHandler)

		authHandler.SetupRoutes(r)
		businessHandler.SetupRoutes(r)
		locationHandler.SetupRoutes(r)
		inviteHandler.SetupRoutes(r)
		positionHandler.SetupRoutes(r)
		shiftHandler.SetupRoutes(r)
		employeeHandler.SetupRoutes(r)
		locationRoleHandler.SetupRoutes(r)
	})

	// Listen
	server := http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.ApiPort),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second, // Max duration reading the entire request
		WriteTimeout:      15 * time.Second, // Max duration writing the response
		IdleTimeout:       60 * time.Second, // Max time to keep connections alive
	}

	// Run ListenAndServe in a Goroutine to not block main
	go func() {
		log.Printf("Server started on port %s", cfg.ApiPort)
		// http.ErrServerClosed check is done since server.Shutdown() makes ListenAndServe()
		// return it, so it is expected.
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Block until signalled
	<-ctx.Done()
	log.Println("Shutdown signal received.")

	// Restore default signal behaviour, this makes Ctrl+C twice force exit.
	stop()

	// Set up shutdown context: this gives in-flight requests time to finish
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}

	log.Println("Server shutdown successfully.")
}
