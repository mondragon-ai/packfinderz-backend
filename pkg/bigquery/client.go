package bigquery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	metadataCheckTimeout = 10 * time.Second
)

type Client struct {
	client    *bigquery.Client
	dataset   *bigquery.Dataset
	projectID string
	tables    []string
	cfg       config.BigQueryConfig
}

var (
	errProjectIDRequired    = errors.New("gcp project id is required")
	errDatasetRequired      = errors.New("bigquery dataset is required")
	errTableNameRequired    = errors.New("bigquery table name is required")
	errClientNotInitialized = errors.New("bigquery client not initialized")
)

type Pinger interface {
	Ping(context.Context) error
}

// NewClient creates a BigQuery client and verifies the configured dataset + tables.
func NewClient(ctx context.Context, gcp config.GCPConfig, cfg config.BigQueryConfig, logg *logger.Logger) (*Client, error) {
	projectID := strings.TrimSpace(gcp.ProjectID)
	if projectID == "" {
		return nil, errProjectIDRequired
	}

	datasetID := strings.TrimSpace(cfg.Dataset)
	if datasetID == "" {
		return nil, errDatasetRequired
	}

	tables := configuredTables(cfg)
	if len(tables) == 0 {
		return nil, errTableNameRequired
	}

	opts := clientOptions(gcp)
	bqClient, err := bigquery.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating bigquery client: %w", err)
	}

	client := &Client{
		client:    bqClient,
		dataset:   bqClient.Dataset(datasetID),
		projectID: projectID,
		tables:    tables,
		cfg:       cfg,
	}

	if err := client.ensureDatasetAndTables(ctx); err != nil {
		_ = bqClient.Close()
		return nil, err
	}

	if logg != nil {
		logg.Info(ctx, "bigquery client initialized")
	}

	return client, nil
}

func clientOptions(gcp config.GCPConfig) []option.ClientOption {
	var opts []option.ClientOption
	switch {
	case strings.TrimSpace(gcp.CredentialsJSON) != "":
		opts = append(opts, option.WithCredentialsJSON([]byte(gcp.CredentialsJSON)))
	case strings.TrimSpace(gcp.ApplicationCredentials) != "":
		opts = append(opts, option.WithCredentialsFile(gcp.ApplicationCredentials))
	}
	return opts
}

func configuredTables(cfg config.BigQueryConfig) []string {
	tables := []string{}
	if trimmed := strings.TrimSpace(cfg.MarketplaceEventsTable); trimmed != "" {
		tables = append(tables, trimmed)
	}
	if trimmed := strings.TrimSpace(cfg.AdEventsTable); trimmed != "" {
		tables = append(tables, trimmed)
	}
	return tables
}

func (c *Client) ensureDatasetAndTables(ctx context.Context) error {
	if c == nil || c.dataset == nil {
		return errClientNotInitialized
	}

	ctx, cancel := context.WithTimeout(ctx, metadataCheckTimeout)
	defer cancel()

	if _, err := c.dataset.Metadata(ctx); err != nil {
		if isNotFound(err) {
			return fmt.Errorf("dataset %q does not exist", c.dataset.DatasetID)
		}
		return fmt.Errorf("checking dataset %q: %w", c.dataset.DatasetID, err)
	}

	for _, name := range c.tables {
		if _, err := c.dataset.Table(name).Metadata(ctx); err != nil {
			if isNotFound(err) {
				return fmt.Errorf("table %q does not exist", name)
			}
			return fmt.Errorf("checking table %q: %w", name, err)
		}
	}

	return nil
}

// Ping verifies the dataset + tables are accessible.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil {
		return errClientNotInitialized
	}
	return c.ensureDatasetAndTables(ctx)
}

// InsertRows sends rows to the given table in the configured dataset.
func (c *Client) InsertRows(ctx context.Context, table string, rows []any) error {
	if c == nil || c.client == nil {
		return errClientNotInitialized
	}
	if strings.TrimSpace(table) == "" {
		return errTableNameRequired
	}
	if len(rows) == 0 {
		return nil
	}

	insertCtx := ctx
	if insertCtx == nil {
		insertCtx = context.Background()
	}

	inserter := c.dataset.Table(strings.TrimSpace(table)).Inserter()
	return inserter.Put(insertCtx, rows)
}

// Query executes SQL against BigQuery and returns the row iterator.
func (c *Client) Query(ctx context.Context, sql string, params []bigquery.QueryParameter) (*bigquery.RowIterator, error) {
	if c == nil || c.client == nil {
		return nil, errClientNotInitialized
	}
	if strings.TrimSpace(sql) == "" {
		return nil, errors.New("sql query is required")
	}
	q := c.client.Query(sql)
	q.Parameters = params
	return q.Read(ctx)
}

// Close releases the BigQuery client.
func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func isNotFound(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr.Code == http.StatusNotFound
	}
	return false
}
