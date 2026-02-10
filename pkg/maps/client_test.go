package maps

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestClientAutocompleteRequest(t *testing.T) {
	const expectedURL = "http://maps.test/v1/places:autocomplete"
	respBody := `{"suggestions":[{"placePrediction":{"placeId":"place_123","text":{"text":"123 Demo St"}}}]}`

	var capturedURL string
	var capturedHeaders http.Header

	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedURL = req.URL.String()
		capturedHeaders = req.Header.Clone()

		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		if payload["input"] != "123 15th st sw" {
			t.Fatalf("unexpected input %q", payload["input"])
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(respBody)),
			Header:     http.Header{},
		}, nil
	})

	httpClient := &http.Client{Transport: rt}
	client, err := NewClient("test-key", WithBaseURL("http://maps.test/v1"), WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	result, err := client.Autocomplete(context.Background(), AutocompleteRequest{
		Input:               "123 15th st sw",
		IncludedRegionCodes: []string{"US"},
		LanguageCode:        "en",
	})
	if err != nil {
		t.Fatalf("autocomplete: %v", err)
	}
	if capturedURL != expectedURL {
		t.Fatalf("unexpected URL %q", capturedURL)
	}
	if capturedHeaders.Get("X-Goog-Api-Key") != "test-key" {
		t.Fatalf("api key header missing")
	}
	if capturedHeaders.Get("X-Goog-FieldMask") != autocompleteFieldMask {
		t.Fatalf("unexpected field mask %q", capturedHeaders.Get("X-Goog-FieldMask"))
	}
	if len(result) != 1 || result[0].PlaceID != "place_123" {
		t.Fatalf("unexpected result %+v", result)
	}
}

func TestClientResolvePlaceRequest(t *testing.T) {
	const expectedURL = "http://maps.test/v1/places/place_123"
	respBody := `{"id":"place_123","formattedAddress":"123 Demo St","location":{"latitude":1.23,"longitude":-4.56},"addressComponents":[{"longText":"123","shortText":"123","types":["street_number"]}]}`

	var capturedURL string
	var capturedHeaders http.Header

	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedURL = req.URL.String()
		capturedHeaders = req.Header.Clone()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(respBody)),
			Header:     http.Header{},
		}, nil
	})

	httpClient := &http.Client{Transport: rt}
	client, err := NewClient("test-key", WithBaseURL("http://maps.test/v1"), WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	details, err := client.ResolvePlace(context.Background(), "place_123")
	if err != nil {
		t.Fatalf("resolve place: %v", err)
	}
	if capturedURL != expectedURL {
		t.Fatalf("unexpected URL %q", capturedURL)
	}
	if capturedHeaders.Get("X-Goog-Api-Key") != "test-key" {
		t.Fatalf("api key header missing")
	}
	if capturedHeaders.Get("X-Goog-FieldMask") != placeResolveFieldMask {
		t.Fatalf("unexpected field mask %q", capturedHeaders.Get("X-Goog-FieldMask"))
	}
	if details.FormattedAddress != "123 Demo St" {
		t.Fatalf("unexpected address %q", details.FormattedAddress)
	}
	if details.Location.Latitude != 1.23 || details.Location.Longitude != -4.56 {
		t.Fatalf("unexpected location %+v", details.Location)
	}
	if len(details.AddressComponents) != 1 || details.AddressComponents[0].LongName != "123" {
		t.Fatalf("unexpected components %+v", details.AddressComponents)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
