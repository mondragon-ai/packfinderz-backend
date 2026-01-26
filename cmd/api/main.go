package main

import (
	"context"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/angelmondragon/packfinderz-backend/api/routes"
	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/internal/cart"
	checkoutsvc "github.com/angelmondragon/packfinderz-backend/internal/checkout"
	"github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	products "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/angelmondragon/packfinderz-backend/pkg/storage/gcs"
)

func main() {
	logg := logger.New(logger.Options{ServiceName: "api"})

	if err := godotenv.Load(); err != nil {
		logg.Warn(context.Background(), ".env file not found, relying on environment")
	}

	cfg, err := config.Load()
	if err != nil {
		logg.Error(context.Background(), "failed to load config", err)
		os.Exit(1)
	}

	logg = logger.New(logger.Options{
		ServiceName: "api",
		Level:       logger.ParseLevel(cfg.App.LogLevel),
		WarnStack:   cfg.App.LogWarnStack,
	})

	dbClient, err := db.New(context.Background(), cfg.DB, logg)
	if err != nil {
		logg.Error(context.Background(), "failed to bootstrap database", err)
		os.Exit(1)
	}
	defer func() {
		if err := dbClient.Close(); err != nil {
			logg.Error(context.Background(), "error closing database", err)
		}
	}()

	if err := migrate.MaybeRunDev(context.Background(), cfg, logg, dbClient); err != nil {
		logg.Error(context.Background(), "failed to run dev migrations", err)
		os.Exit(1)
	}

	redisClient, err := redis.New(context.Background(), cfg.Redis, logg)
	if err != nil {
		logg.Error(context.Background(), "failed to bootstrap redis", err)
		os.Exit(1)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			logg.Error(context.Background(), "error closing redis", err)
		}
	}()

	sessionManager, err := session.NewManager(redisClient, cfg.JWT)
	if err != nil {
		logg.Error(context.Background(), "failed to create session manager", err)
		os.Exit(1)
	}

	gcsClient, err := gcs.NewClient(context.Background(), cfg.GCS, cfg.GCP, logg)
	if err != nil {
		logg.Error(context.Background(), "failed to bootstrap gcs", err)
		os.Exit(1)
	}
	defer func() {
		if err := gcsClient.Close(); err != nil {
			logg.Error(context.Background(), "error closing gcs client", err)
		}
	}()

	usersRepo := users.NewRepository(dbClient.DB())
	membershipsRepo := memberships.NewRepository(dbClient.DB())
	authService, err := auth.NewService(auth.ServiceParams{
		UserRepo:        usersRepo,
		MembershipsRepo: membershipsRepo,
		SessionManager:  sessionManager,
		JWTConfig:       cfg.JWT,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create auth service", err)
		os.Exit(1)
	}

	registerService, err := auth.NewRegisterService(auth.RegisterServiceParams{
		DB:             dbClient,
		PasswordConfig: cfg.Password,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create register service", err)
		os.Exit(1)
	}

	switchService, err := auth.NewSwitchStoreService(auth.SwitchStoreServiceParams{
		MembershipsRepo: membershipsRepo,
		SessionManager:  sessionManager,
		JWTConfig:       cfg.JWT,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create switch store service", err)
		os.Exit(1)
	}

	storeRepo := stores.NewRepository(dbClient.DB())
	storeService, err := stores.NewService(storeRepo, membershipsRepo, usersRepo, cfg.Password)
	if err != nil {
		logg.Error(context.Background(), "failed to create store service", err)
		os.Exit(1)
	}

	mediaRepo := media.NewRepository(dbClient.DB())
	mediaService, err := media.NewService(
		mediaRepo,
		membershipsRepo,
		gcsClient,
		cfg.GCS.BucketName,
		cfg.GCS.UploadURLExpiry,
		cfg.GCS.DownloadURLExpiry,
	)
	if err != nil {
		logg.Error(context.Background(), "failed to create media service", err)
		os.Exit(1)
	}

	productRepo := products.NewRepository(dbClient.DB())
	productService, err := products.NewService(productRepo, dbClient, storeRepo, membershipsRepo, mediaRepo)
	if err != nil {
		logg.Error(context.Background(), "failed to create product service", err)
		os.Exit(1)
	}

	cartRepo := cart.NewRepository(dbClient.DB())
	cartService, err := cart.NewService(
		cartRepo,
		dbClient,
		storeService,
		productRepo,
	)
	if err != nil {
		logg.Error(context.Background(), "failed to create cart service", err)
		os.Exit(1)
	}

	outboxRepo := outbox.NewRepository(dbClient.DB())
	outboxPublisher := outbox.NewService(outboxRepo, logg)

	ordersRepo := orders.NewRepository(dbClient.DB())
	checkoutService, err := checkoutsvc.NewService(
		dbClient,
		cartRepo,
		ordersRepo,
		storeService,
		productRepo,
		nil,
		outboxPublisher,
	)
	if err != nil {
		logg.Error(context.Background(), "failed to create checkout service", err)
		os.Exit(1)
	}

	licenseService, err := licenses.NewService(
		licenses.NewRepository(dbClient.DB()),
		media.NewRepository(dbClient.DB()),
		membershipsRepo,
		gcsClient,
		cfg.GCS.BucketName,
		cfg.GCS.DownloadURLExpiry,
		storeRepo,
		dbClient,
		outboxPublisher,
	)
	if err != nil {
		logg.Error(context.Background(), "failed to create license service", err)
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.App.Port
	}
	addr := ":" + port
	id := os.Getenv("DYNO")
	if id == "" {
		id = "local"
	}
	ctx := logg.WithFields(context.Background(), map[string]any{
		"env":      cfg.App.Env,
		"addr":     addr,
		"instance": id,
	})
	logg.Info(ctx, "starting api server")

	server := &http.Server{
		Addr: addr,
		Handler: routes.NewRouter(
			cfg,
			logg,
			dbClient,
			redisClient,
			gcsClient,
			sessionManager,
			authService,
			registerService,
			switchService,
			storeService,
			mediaService,
			licenseService,
			productService,
			checkoutService,
			cartService,
			ordersRepo,
		),
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logg.Error(ctx, "api server stopped unexpectedly", err)
		os.Exit(1)
	}
}
