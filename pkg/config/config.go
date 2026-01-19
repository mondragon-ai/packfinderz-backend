package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	App          AppConfig
	Service      ServiceConfig
	DB           DBConfig
	Redis        RedisConfig
	JWT          JWTConfig
	FeatureFlags FeatureFlagsConfig
	OpenAI       OpenAIConfig
	GoogleMaps   GoogleMapsConfig
	GCP          GCPConfig
	GCS          GCSConfig
	Media        MediaConfig
	PubSub       PubSubConfig
	Stripe       StripeConfig
	Sendgrid     SendgridConfig
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process(EnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := cfg.DB.ensureDSN(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

type AppConfig struct {
	Env  string `required:"true"`
	Port string `required:"true"`
}

type ServiceConfig struct {
	Kind string
}

type DBConfig struct {
	DSN            string
	Driver         string `default:"postgres"`
	LegacyHost     string `envconfig:"DB_HOST"`
	LegacyPort     int    `envconfig:"DB_PORT" default:"5432"`
	LegacyUser     string `envconfig:"DB_USER"`
	LegacyPassword string `envconfig:"DB_PASSWORD"`
	LegacyName     string `envconfig:"DB_NAME"`
	LegacySSLMode  string `envconfig:"DB_SSLMODE" default:"disable"`
}

type RedisConfig struct {
	URL      string `required:"true"`
	Address  string
	Password string
	DB       int `default:"0"`
}

type JWTConfig struct {
	Secret            string `equired:"true"`
	Issuer            string `required:"true"`
	ExpirationMinutes int    `envconfig:"PACKFINDERZ_JWT_EXPIRATION_MINUTES" required:"true"`
}

type FeatureFlagsConfig struct {
	UseSQLite     bool   `envconfig:"PACKFINDERZ_USE_SQLITE" default:"false"`
	AutoMigrate   bool   `envconfig:"PACKFINDERZ_AUTO_MIGRATE" default:"false"`
	AVScan        string `envconfig:"PACKFINDERZ_AV_SCAN" default:"off"`
	GCSAccessMode string `envconfig:"PACKFINDERZ_GCS_ACCESS_MODE" default:"public"`
}

type OpenAIConfig struct {
	APIKey string `envconfig:"PACKFINDERZ_OPENAI_API_KEY"`
}

type GoogleMapsConfig struct {
	APIKey string `envconfig:"PACKFINDERZ_GOOGLE_MAPS_API_KEY"`
}

type GCPConfig struct {
	ProjectID              string `envconfig:"PACKFINDERZ_GCP_PROJECT_ID" required:"true"`
	CredentialsJSON        string `envconfig:"PACKFINDERZ_GCP_CREDENTIALS_JSON"`
	ApplicationCredentials string `envconfig:"PACKFINDERZ_GOOGLE_APPLICATION_CREDENTIALS"`
}

type GCSConfig struct {
	BucketName        string        `envconfig:"PACKFINDERZ_GCS_BUCKET_NAME" required:"true"`
	UploadURLExpiry   time.Duration `envconfig:"PACKFINDERZ_GCS_UPLOAD_URL_EXPIRY" required:"true"`
	DownloadURLExpiry time.Duration `envconfig:"PACKFINDERZ_GCS_DOWNLOAD_URL_EXPIRY" required:"true"`
}

type MediaConfig struct {
	MaxUploadMB     int    `envconfig:"PACKFINDERZ_MAX_UPLOAD_MB" default:"200"`
	ImageMaxWidth   int    `envconfig:"PACKFINDERZ_MEDIA_IMAGE_MAX_WIDTH" default:"1920"`
	ImageMaxHeight  int    `envconfig:"PACKFINDERZ_MEDIA_IMAGE_MAX_HEIGHT" default:"1080"`
	ImageQuality    int    `envconfig:"PACKFINDERZ_MEDIA_IMAGE_QUALITY" default:"80"`
	VideoCRF        int    `envconfig:"PACKFINDERZ_MEDIA_VIDEO_CRF" default:"23"`
	VideoPreset     string `envconfig:"PACKFINDERZ_MEDIA_VIDEO_PRESET" default:"medium"`
	VideoMaxBitrate string `envconfig:"PACKFINDERZ_MEDIA_VIDEO_MAX_BITRATE" default:"8M"`
	PDFQuality      string `envconfig:"PACKFINDERZ_MEDIA_PDF_QUALITY" default:"ebook"`
	PDFDPI          int    `envconfig:"PACKFINDERZ_MEDIA_PDF_DPI" default:"150"`
}

type PubSubConfig struct {
	MediaTopic          string `envconfig:"PACKFINDERZ_PUBSUB_MEDIA_TOPIC" required:"true"`
	MediaSubscription   string `envconfig:"PACKFINDERZ_PUBSUB_MEDIA_SUBSCRIPTION" required:"true"`
	OrdersTopic         string `envconfig:"PACKFINDERZ_PUBSUB_ORDERS_TOPIC" required:"true"`
	OrdersSubscription  string `envconfig:"PACKFINDERZ_PUBSUB_ORDERS_SUBSCRIPTION" required:"true"`
	BillingTopic        string `envconfig:"PACKFINDERZ_PUBSUB_BILLING_TOPIC" required:"true"`
	BillingSubscription string `envconfig:"PACKFINDERZ_PUBSUB_BILLING_SUBSCRIPTION" required:"true"`
}

type StripeConfig struct {
	APIKey        string `envconfig:"PACKFINDERZ_STRIPE_API_KEY"`
	WebhookSecret string `envconfig:"PACKFINDERZ_STRIPE_WEBHOOK_SECRET"`
}

type SendgridConfig struct {
	APIKey      string `envconfig:"PACKFINDERZ_SENDGRID_API_KEY"`
	DefaultFrom string `envconfig:"PACKFINDERZ_SENDGRID_FROM_EMAIL"`
}

func (db *DBConfig) ensureDSN() error {
	if db.DSN != "" {
		return nil
	}

	missing := []string{}
	legacyValues := map[string]string{
		EnvDBHost: db.LegacyHost,
		EnvDBUser: db.LegacyUser,
		EnvDBName: db.LegacyName,
	}
	for _, env := range legacyDBEnvVars {
		if legacyValues[env] == "" {
			missing = append(missing, env)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("either %s or %s are required", EnvDBDSN, strings.Join(missing, ", "))
	}

	userInfo := url.User(db.LegacyUser)
	if db.LegacyPassword != "" {
		userInfo = url.UserPassword(db.LegacyUser, db.LegacyPassword)
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   userInfo,
		Host:   fmt.Sprintf("%s:%d", db.LegacyHost, db.LegacyPort),
		Path:   db.LegacyName,
	}

	if db.LegacySSLMode != "" {
		q := u.Query()
		q.Set("sslmode", db.LegacySSLMode)
		u.RawQuery = q.Encode()
	}

	db.DSN = u.String()
	return nil
}
