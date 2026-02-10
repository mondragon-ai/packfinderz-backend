package maps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

const (
	defaultBaseURL              = "https://places.googleapis.com/v1"
	autocompleteFieldMask       = "suggestions.placePrediction.placeId,suggestions.placePrediction.text"
	placeResolveFieldMask       = "id,formattedAddress,location,addressComponents"
	requestBodyReadLimit  int64 = 1024
)

var (
	errAPIKeyRequired = errors.New("google maps api key is required")
)

// Client wraps the Google Maps Places APIs used for address guidance.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// Option configures optional client behavior.
type Option func(*Client)

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithBaseURL overrides the configured Places base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		trimmed := strings.TrimSpace(baseURL)
		if trimmed != "" {
			c.baseURL = trimmed
		}
	}
}

// NewClient builds the Google Maps client given an API key.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	trimmedKey := strings.TrimSpace(apiKey)
	if trimmedKey == "" {
		return nil, errAPIKeyRequired
	}

	client := &Client{
		apiKey:     trimmedKey,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}

	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if client.baseURL == "" {
		client.baseURL = defaultBaseURL
	}

	return client, nil
}

// AutocompleteRequest describes the payload sent to the Places autocomplete API.
type AutocompleteRequest struct {
	Input               string   `json:"input"`
	IncludedRegionCodes []string `json:"includedRegionCodes,omitempty"`
	LanguageCode        string   `json:"languageCode,omitempty"`
}

// AutocompleteSuggestion holds the mapped data returned by the autocomplete API.
type AutocompleteSuggestion struct {
	PlaceID     string
	Description string
}

// PlaceDetails represents the normalized data returned by the place-details API.
type PlaceDetails struct {
	PlaceID           string
	FormattedAddress  string
	Location          LatLng
	AddressComponents []AddressComponent
}

// LatLng is the latitude/longitude pair returned by Google.
type LatLng struct {
	Latitude  float64
	Longitude float64
}

// AddressComponent mirrors Google&apos;s address component payload.
type AddressComponent struct {
	LongName  string
	ShortName string
	Types     []string
}

// Autocomplete queries suggested places based on partial input.
func (c *Client) Autocomplete(ctx context.Context, req AutocompleteRequest) ([]AutocompleteSuggestion, error) {
	if c == nil {
		return nil, pkgerrors.New(pkgerrors.CodeDependency, "google maps client not configured")
	}
	if strings.TrimSpace(req.Input) == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "autocomplete input is required")
	}

	url := c.buildURL("places:autocomplete")
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "marshal autocomplete request")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "build autocomplete request")
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Goog-Api-Key", c.apiKey)
	httpReq.Header.Set("X-Goog-FieldMask", autocompleteFieldMask)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "execute autocomplete request")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, requestBodyReadLimit))
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg))), "autocomplete request failed")
	}

	var apiResp struct {
		Suggestions []struct {
			Prediction struct {
				PlaceID string `json:"placeId"`
				Text    struct {
					Text string `json:"text"`
				} `json:"text"`
			} `json:"placePrediction"`
		} `json:"suggestions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "decode autocomplete response")
	}

	suggestions := make([]AutocompleteSuggestion, 0, len(apiResp.Suggestions))
	for _, s := range apiResp.Suggestions {
		suggestions = append(suggestions, AutocompleteSuggestion{
			PlaceID:     s.Prediction.PlaceID,
			Description: s.Prediction.Text.Text,
		})
	}

	return suggestions, nil
}

// ResolvePlace fetches the canonical place data for the provided place ID.
func (c *Client) ResolvePlace(ctx context.Context, placeID string) (*PlaceDetails, error) {
	if c == nil {
		return nil, pkgerrors.New(pkgerrors.CodeDependency, "google maps client not configured")
	}
	trimmed := strings.TrimSpace(placeID)
	if trimmed == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "place ID is required")
	}

	url := fmt.Sprintf("%s/places/%s", strings.TrimRight(c.baseURL, "/"), url.PathEscape(trimmed))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "build place resolve request")
	}

	httpReq.Header.Set("X-Goog-Api-Key", c.apiKey)
	httpReq.Header.Set("X-Goog-FieldMask", placeResolveFieldMask)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "execute place resolve request")
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, requestBodyReadLimit))
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg))), "place resolve request failed")
	}

	var apiResp struct {
		ID               string `json:"id"`
		FormattedAddress string `json:"formattedAddress"`
		Location         struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"location"`
		AddressComponents []struct {
			LongName  string   `json:"longText"`
			ShortName string   `json:"shortText"`
			Types     []string `json:"types"`
		} `json:"addressComponents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "decode place resolve response")
	}

	components := make([]AddressComponent, 0, len(apiResp.AddressComponents))
	for _, comp := range apiResp.AddressComponents {
		components = append(components, AddressComponent{
			LongName:  comp.LongName,
			ShortName: comp.ShortName,
			Types:     comp.Types,
		})
	}

	return &PlaceDetails{
		PlaceID:          apiResp.ID,
		FormattedAddress: apiResp.FormattedAddress,
		Location: LatLng{
			Latitude:  apiResp.Location.Latitude,
			Longitude: apiResp.Location.Longitude,
		},
		AddressComponents: components,
	}, nil
}

func (c *Client) buildURL(path string) string {
	trimmed := strings.TrimRight(c.baseURL, "/")
	path = strings.TrimLeft(path, "/")
	return fmt.Sprintf("%s/%s", trimmed, path)
}
