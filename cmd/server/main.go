package main

import (
	"fmt"
	"log"
	"net/http"
	_ "time/tzdata"

	_ "github.com/dgrco/quikslate/docs"
	"github.com/dgrco/quikslate/internal/config"
	"github.com/dgrco/quikslate/internal/database"
	"github.com/dgrco/quikslate/internal/handler"
	"github.com/dgrco/quikslate/internal/infra/repo"
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
			w.Write([]byte("Hi from Quikslate :)"))
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
	log.Printf("Server started on port %s", cfg.ApiPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", cfg.ApiPort), r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
