// Package routing is the IO half of services/api/app/journey/routing.py:
// opening a socket to the configured private Valhalla, and nothing else.
// Shaping the answer is internal/domain/valhalla; deciding what to ask is
// internal/domain/itinerary.
package routing

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"mobile/services/core/internal/domain/itinerary"
	"mobile/services/core/internal/domain/journey"
	"mobile/services/core/internal/domain/valhalla"
	"mobile/services/core/internal/pyjson"
)

const timeout = 12 * time.Second

var (
	errRedirect = errors.New("routing_unavailable")
	errConfig   = errors.New("invalid_routing_configuration")
)

// Client is ValhallaProvider. A nil *Client is configured_provider() finding
// no configuration, which is an answer of its own.
type Client struct {
	baseURL      string
	graphVersion string
	http         *http.Client
}

// Configured is configured_provider: nil when either env var is missing or
// the URL/version fail construction.
func Configured() itinerary.Router {
	base := os.Getenv("MOBILE_VALHALLA_URL")
	version := os.Getenv("MOBILE_ROUTING_GRAPH_VERSION")
	if base == "" || version == "" {
		return nil
	}
	client, err := newClient(base, version)
	if err != nil {
		return nil
	}
	return client
}

func newClient(baseURL, graphVersion string) (*Client, error) {
	if strings.TrimSpace(graphVersion) == "" {
		return nil, errConfig
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, errConfig
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errConfig
	}
	if parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errConfig
	}
	transport := &http.Transport{Proxy: func(*http.Request) (*url.URL, error) { return nil, nil }}
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		graphVersion: graphVersion,
		http: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errRedirect
			},
		},
	}, nil
}

// GraphVersion is provider.graph_version.
func (c *Client) GraphVersion() string { return c.graphVersion }

// Route is ValhallaProvider.route.
func (c *Client) Route(points []itinerary.Point, mode string) ([]valhalla.Leg, error) {
	if len(points) < 2 {
		return []valhalla.Leg{}, nil
	}
	costing, ok := valhalla.Costing(mode)
	if !ok {
		return nil, fmt.Errorf("routing: unknown mode %q", mode)
	}
	locations := make(pyjson.List, len(points))
	for i, point := range points {
		row := pyjson.NewOrderedMap()
		row.Set("lat", jsonNumber(point.Lat))
		row.Set("lon", jsonNumber(point.Lng))
		row.Set("type", pyjson.String("break"))
		locations[i] = row
	}
	payload := pyjson.NewOrderedMap()
	payload.Set("locations", locations)
	payload.Set("costing", pyjson.String(costing))
	payload.Set("units", pyjson.String("kilometers"))
	payload.Set("shape_format", pyjson.String("polyline6"))
	options := pyjson.NewOrderedMap()
	options.Set("units", pyjson.String("kilometers"))
	payload.Set("directions_options", options)
	body, err := c.post("route", payload)
	if err != nil {
		return nil, err
	}
	legs, err := valhalla.ShapeRoute(body, len(points))
	if err != nil {
		return nil, valhalla.ErrRoutingUnavailable
	}
	return legs, nil
}

// Matrix is ValhallaProvider.matrix.
func (c *Client) Matrix(points []itinerary.Point, mode string) (journey.Matrix, error) {
	costing, ok := valhalla.Costing(mode)
	if !ok {
		return nil, fmt.Errorf("routing: unknown mode %q", mode)
	}
	listed := pointObjects(points)
	payload := pyjson.NewOrderedMap()
	payload.Set("sources", listed)
	payload.Set("targets", listed)
	payload.Set("costing", pyjson.String(costing))
	payload.Set("units", pyjson.String("kilometers"))
	payload.Set("verbose", pyjson.Bool(true))
	body, err := c.post("sources_to_targets", payload)
	if err != nil {
		return nil, err
	}
	matrix, err := valhalla.ShapeMatrix(body, len(points))
	if err != nil {
		return nil, valhalla.ErrRoutingUnavailable
	}
	return matrix, nil
}

func (c *Client) post(action string, payload pyjson.Value) (any, error) {
	encoded, err := pyjson.Dumps(payload)
	if err != nil {
		return nil, valhalla.ErrRoutingUnavailable
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/"+action, bytes.NewReader(encoded))
	if err != nil {
		return nil, valhalla.ErrRoutingUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, valhalla.ErrRoutingUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, valhalla.ErrRoutingUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, int64(valhalla.MaxResponseBytes)+1))
	if err != nil || len(raw) > valhalla.MaxResponseBytes {
		return nil, valhalla.ErrRoutingUnavailable
	}
	value, err := pyjson.Loads(raw)
	if err != nil {
		return nil, valhalla.ErrRoutingUnavailable
	}
	obj, ok := value.(*pyjson.OrderedMap)
	if !ok {
		return nil, valhalla.ErrRoutingUnavailable
	}
	if _, found := obj.Get("error_code"); found {
		return nil, valhalla.ErrRoutingUnavailable
	}
	return asAny(obj), nil
}
