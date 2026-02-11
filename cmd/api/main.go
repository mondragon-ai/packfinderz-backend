package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/angelmondragon/packfinderz-backend/api/routes"
	"github.com/angelmondragon/packfinderz-backend/internal/address"
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
	"github.com/angelmondragon/packfinderz-backend/internal/paymentmethods"
	products "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/internal/squarecustomers"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	squarewebhook "github.com/angelmondragon/packfinderz-backend/internal/webhooks/square"
	"github.com/angelmondragon/packfinderz-backend/pkg/auth/session"
	"github.com/angelmondragon/packfinderz-backend/pkg/bigquery"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/maps"
	"github.com/angelmondragon/packfinderz-backend/pkg/migrate"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/redis"
	"github.com/angelmondragon/packfinderz-backend/pkg/square"
	gcs "github.com/angelmondragon/packfinderz-backend/pkg/storage/gcs"
	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()
	logg := logger.New(logger.Options{ServiceName: "api"})

	_ = godotenv.Load()

	cfg, err := config.Load()
	requireResource(ctx, logg, "config", err)

	logg = logger.New(logger.Options{
		ServiceName: "api",
		Level:       logger.ParseLevel(cfg.App.LogLevel),
		WarnStack:   cfg.App.LogWarnStack,
	})

	squareClient, err := square.NewClient(context.Background(), cfg.Square, logg)
	requireResource(ctx, logg, "square client", err)

	mapsClient, err := maps.NewClient(cfg.GoogleMaps.APIKey)
	requireResource(ctx, logg, "google maps client", err)
	addressService := address.NewService(mapsClient)

	squareCustomerService := squarecustomers.NewService(squareClient)

	squareSubsClient := subscriptions.NewSquareClient(squareClient, cfg.Square.LocationID)

	dbClient, err := db.New(context.Background(), cfg.DB, logg)
	requireResource(ctx, logg, "database", err)
	defer func() {
		if err := dbClient.Close(); err != nil {
			logg.Error(ctx, "failed to close database client", err)
		}
	}()

	requireResource(ctx, logg, "migrations", migrate.MaybeRunDev(context.Background(), cfg, logg, dbClient))

	redisClient, err := redis.New(context.Background(), cfg.Redis, logg)
	requireResource(ctx, logg, "redis", err)

	defer func() {
		if err := redisClient.Close(); err != nil {
			logg.Error(ctx, "failed to close redis client", err)
		}
	}()

	sessionManager, err := session.NewManager(redisClient, cfg.JWT)
	requireResource(ctx, logg, "session manager", err)

	gcsClient, err := gcs.NewClient(context.Background(), cfg.GCS, cfg.GCP, logg)
	requireResource(ctx, logg, "gcs", err)
	defer func() {
		if err := gcsClient.Close(); err != nil {
			logg.Error(ctx, "failed to close gcs client", err)
		}
	}()

	bqClient, err := bigquery.NewClient(context.Background(), cfg.GCP, cfg.BigQuery, logg)
	requireResource(ctx, logg, "bigquery", err)
	defer func() {
		if err := bqClient.Close(); err != nil {
			logg.Error(ctx, "failed to close bigquery client", err)
		}
	}()

	analyticsService, err := analytics.NewService(bqClient, cfg.GCP.ProjectID, cfg.BigQuery.Dataset, cfg.BigQuery.MarketplaceEventsTable)
	requireResource(ctx, logg, "analytics service", err)

	usersRepo := users.NewRepository(dbClient.DB())
	membershipsRepo := memberships.NewRepository(dbClient.DB())
	storeRepo := stores.NewRepository(dbClient.DB())
	authService, err := auth.NewService(auth.ServiceParams{
		UserRepo:        usersRepo,
		MembershipsRepo: membershipsRepo,
		SessionManager:  sessionManager,
		JWTConfig:       cfg.JWT,
	})
	requireResource(ctx, logg, "auth service", err)

	registerService, err := auth.NewRegisterService(auth.RegisterServiceParams{
		DB:                    dbClient,
		PasswordConfig:        cfg.Password,
		SquareCustomerService: squareCustomerService,
	})
	requireResource(ctx, logg, "register service", err)
	adminRegisterService, err := auth.NewAdminRegisterService(auth.AdminRegisterServiceParams{
		DB:             dbClient,
		PasswordConfig: cfg.Password,
	})
	requireResource(ctx, logg, "admin register service", err)

	switchService, err := auth.NewSwitchStoreService(auth.SwitchStoreServiceParams{
		MembershipsRepo: membershipsRepo,
		SessionManager:  sessionManager,
		JWTConfig:       cfg.JWT,
	})
	requireResource(ctx, logg, "switch store service", err)

	billingRepo := billing.NewRepository(dbClient.DB())
	billingService, err := billing.NewService(billing.ServiceParams{
		Repo: billingRepo,
	})
	requireResource(ctx, logg, "billing service", err)

	paymentMethodService, err := paymentmethods.NewService(paymentmethods.ServiceParams{
		BillingRepo:       billingRepo,
		StoreLoader:       storeRepo,
		SquareClient:      squareClient,
		TransactionRunner: dbClient,
	})
	requireResource(ctx, logg, "payment method service", err)

	subscriptionsService, err := subscriptions.NewService(subscriptions.ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         storeRepo,
		SquareClient:      squareSubsClient,
		DefaultPriceID:    cfg.Square.SubscriptionPlanID,
		TransactionRunner: dbClient,
	})
	requireResource(ctx, logg, "subscription service", err)

	squareWebhookService, err := squarewebhook.NewService(squarewebhook.ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         storeRepo,
		SquareClient:      squareSubsClient,
		TransactionRunner: dbClient,
	})
	requireResource(ctx, logg, "square webhook service", err)

	squareWebhookGuard, err := squarewebhook.NewIdempotencyGuard(redisClient, cfg.Eventing.OutboxIdempotencyTTL, "square-webhook")
	requireResource(ctx, logg, "square webhook guard", err)

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
	requireResource(ctx, logg, "media service", err)
	attachmentReconciler, err := media.NewAttachmentReconciler(mediaAttachmentRepo, mediaRepo)
	requireResource(ctx, logg, "attachment reconciler", err)
	storeService, err := stores.NewService(stores.ServiceParams{
		Repo:                 storeRepo,
		Memberships:          membershipsRepo,
		Users:                usersRepo,
		PasswordCfg:          cfg.Password,
		TransactionRunner:    dbClient,
		AttachmentReconciler: attachmentReconciler,
	})
	requireResource(ctx, logg, "store service", err)

	productRepo := products.NewRepository(dbClient.DB())
	productService, err := products.NewService(productRepo, dbClient, storeRepo, membershipsRepo, mediaRepo, attachmentReconciler, mediaService)
	requireResource(ctx, logg, "product service", err)

	cartRepo := cart.NewRepository(dbClient.DB())
	cartTokenValidator, err := cart.NewJWTAttributionTokenValidator(cfg.JWT)
	requireResource(ctx, logg, "cart token validator", err)
	cartService, err := cart.NewService(
		cartRepo,
		dbClient,
		storeService,
		productRepo,
		cart.NoopPromoLoader(),
		cartTokenValidator,
	)
	requireResource(ctx, logg, "cart service", err)

	outboxRepo := outbox.NewRepository(dbClient.DB())
	outboxPublisher := outbox.NewService(outboxRepo, logg)

	ledgerRepo := ledger.NewRepository(dbClient.DB())
	ledgerService, err := ledger.NewService(ledgerRepo)
	requireResource(ctx, logg, "ledger service", err)

	ordersRepo := orders.NewRepository(dbClient.DB())
	ordersService, err := orders.NewService(ordersRepo, dbClient, outboxPublisher, orders.NewInventoryReleaser(), orders.NewInventoryReserver(), ledgerService)
	requireResource(ctx, logg, "orders service", err)

	notificationsRepo := notifications.NewRepository(dbClient.DB())
	notificationsService, err := notifications.NewService(notificationsRepo)
	requireResource(ctx, logg, "notifications service", err)
	checkoutService, err := checkoutsvc.NewService(
		dbClient,
		cartRepo,
		ordersRepo,
		storeService,
		productRepo,
		nil,
		outboxPublisher,
		cfg.FeatureFlags.AllowACH,
	)
	requireResource(ctx, logg, "checkout service", err)
	checkoutRepo := checkoutsvc.NewRepository(dbClient.DB(), ordersRepo)

	licenseService, err := licenses.NewService(
		licenses.NewRepository(dbClient.DB()),
		mediaRepo,
		membershipsRepo,
		attachmentReconciler,
		gcsClient,
		cfg.GCS.BucketName,
		cfg.GCS.DownloadURLExpiry,
		storeRepo,
		dbClient,
		outboxPublisher,
	)
	requireResource(ctx, logg, "license service", err)

	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.App.Port
	}
	addr := ":" + port
	id := os.Getenv("DYNO")
	if id == "" {
		id = "local"
	}
	serverCtx := logg.WithFields(ctx, map[string]any{
		"env":      cfg.App.Env,
		"addr":     addr,
		"instance": id,
	})
	logg.Info(serverCtx, "api ready")

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
			storeRepo,
			membershipsRepo,
			squareCustomerService,
			mediaService,
			licenseService,
			productService,
			checkoutService,
			checkoutRepo,
			cartService,
			notificationsService,
			ordersRepo,
			ordersService,
			subscriptionsService,
			paymentMethodService,
			billingService,
			billingService,
			squareClient,
			squareWebhookService,
			squareWebhookGuard,
			addressService,
		),
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logg.Error(serverCtx, "api not working", err)
		os.Exit(1)
	}
}

func requireResource(ctx context.Context, logg *logger.Logger, resource string, err error) {
	if err == nil {
		return
	}
	logg.Error(ctx, fmt.Sprintf("resource not working: %s", resource), err)
	os.Exit(1)
}
