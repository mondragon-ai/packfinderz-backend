package main

import (
	"context"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/angelmondragon/packfinderz-backend/api/routes"
	"github.com/angelmondragon/packfinderz-backend/internal/analytics"
	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/internal/cart"
	checkoutsvc "github.com/angelmondragon/packfinderz-backend/internal/checkout"
	"github.com/angelmondragon/packfinderz-backend/internal/ledger"
	"github.com/angelmondragon/packfinderz-backend/internal/licenses"
	"github.com/angelmondragon/packfinderz-backend/internal/media"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/notifications"
	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	products "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	stripewebhook "github.com/angelmondragon/packfinderz-backend/internal/webhooks/stripe"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/bigquery"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/angelmondragon/packfinderz-backend/pkg/storage/gcs"
	"github.com/angelmondragon/packfinderz-backend/pkg/stripe"
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

	stripeClient, err := stripe.NewClient(context.Background(), cfg.Stripe, logg)
	if err != nil {
		logg.Error(context.Background(), "failed to bootstrap stripe client", err)
		os.Exit(1)
	}

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

	bqClient, err := bigquery.NewClient(context.Background(), cfg.GCP, cfg.BigQuery, logg)
	if err != nil {
		logg.Error(context.Background(), "failed to bootstrap bigquery", err)
		os.Exit(1)
	}
	defer func() {
		if err := bqClient.Close(); err != nil {
			logg.Error(context.Background(), "error closing bigquery client", err)
		}
	}()

	analyticsService, err := analytics.NewService(bqClient, cfg.GCP.ProjectID, cfg.BigQuery.Dataset, cfg.BigQuery.MarketplaceEventsTable)
	if err != nil {
		logg.Error(context.Background(), "failed to create analytics service", err)
		os.Exit(1)
	}

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
	adminRegisterService, err := auth.NewAdminRegisterService(auth.AdminRegisterServiceParams{
		DB:             dbClient,
		PasswordConfig: cfg.Password,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create admin register service", err)
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

	billingRepo := billing.NewRepository(dbClient.DB())
	billingService, err := billing.NewService(billing.ServiceParams{
		Repo: billingRepo,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create billing service", err)
		os.Exit(1)
	}

	subscriptionsService, err := subscriptions.NewService(subscriptions.ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         storeRepo,
		StripeClient:      subscriptions.NewStripeClient(stripeClient),
		DefaultPriceID:    cfg.Stripe.SubscriptionPriceID,
		TransactionRunner: dbClient,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create subscription service", err)
		os.Exit(1)
	}

	stripeWebhookService, err := stripewebhook.NewService(stripewebhook.ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         storeRepo,
		StripeClient:      subscriptions.NewStripeClient(stripeClient),
		TransactionRunner: dbClient,
	})
	if err != nil {
		logg.Error(context.Background(), "failed to create stripe webhook service", err)
		os.Exit(1)
	}

	stripeWebhookGuard, err := stripewebhook.NewIdempotencyGuard(redisClient, cfg.Eventing.OutboxIdempotencyTTL, "stripe-webhook")
	if err != nil {
		logg.Error(context.Background(), "failed to create stripe webhook guard", err)
		os.Exit(1)
	}

	mediaRepo := media.NewRepository(dbClient.DB())
	mediaAttachmentRepo := media.NewMediaAttachmentRepository(dbClient.DB())
	mediaService, err := media.NewService(
		mediaRepo,
		membershipsRepo,
		mediaAttachmentRepo,
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
		cart.NoopPromoLoader(),
	)
	if err != nil {
		logg.Error(context.Background(), "failed to create cart service", err)
		os.Exit(1)
	}

	outboxRepo := outbox.NewRepository(dbClient.DB())
	outboxPublisher := outbox.NewService(outboxRepo, logg)

	ledgerRepo := ledger.NewRepository(dbClient.DB())
	ledgerService, err := ledger.NewService(ledgerRepo)
	if err != nil {
		logg.Error(context.Background(), "failed to create ledger service", err)
		os.Exit(1)
	}

	ordersRepo := orders.NewRepository(dbClient.DB())
	ordersService, err := orders.NewService(ordersRepo, dbClient, outboxPublisher, orders.NewInventoryReleaser(), orders.NewInventoryReserver(), ledgerService)
	if err != nil {
		logg.Error(context.Background(), "failed to create orders service", err)
		os.Exit(1)
	}

	notificationsRepo := notifications.NewRepository(dbClient.DB())
	notificationsService, err := notifications.NewService(notificationsRepo)
	if err != nil {
		logg.Error(context.Background(), "failed to create notifications service", err)
		os.Exit(1)
	}
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
			bqClient,
			sessionManager,
			analyticsService,
			authService,
			registerService,
			adminRegisterService,
			switchService,
			storeService,
			mediaService,
			licenseService,
			productService,
			checkoutService,
			cartService,
			notificationsService,
			ordersRepo,
			ordersService,
			subscriptionsService,
			billingService,
			stripeClient,
			stripeWebhookService,
			stripeWebhookGuard,
		),
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logg.Error(ctx, "api server stopped unexpectedly", err)
		os.Exit(1)
	}
}
