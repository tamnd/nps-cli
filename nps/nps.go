// Package nps is the library behind the nps command line:
// the HTTP client, request shaping, and the typed data models for the
// National Park Service (NPS) public API.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package nps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Host is the NPS API host. Used in domain.go for the driver's host list.
const Host = "developer.nps.gov"

// Config holds all tunable knobs for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	APIKey    string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns sensible defaults for the NPS API.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://developer.nps.gov/api/v1",
		UserAgent: "nps-cli/0.1.0 (github.com/tamnd/nps-cli)",
		APIKey:    "DEMO_KEY",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Client talks to the NPS API over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client using the provided Config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// get appends api_key to the given URL and fetches the body.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("api_key", c.cfg.APIKey)
	u.RawQuery = q.Encode()
	return c.fetch(ctx, u.String())
}

// fetch executes the request with pacing and retry.
func (c *Client) fetch(ctx context.Context, endpoint string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, endpoint)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", endpoint, lastErr)
}

func (c *Client) do(ctx context.Context, endpoint string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least cfg.Rate has elapsed since the last request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// apiResponse is the common envelope for all NPS API list responses.
type apiResponse[T any] struct {
	Total string `json:"total"`
	Limit string `json:"limit"`
	Start string `json:"start"`
	Data  []T    `json:"data"`
}

// --- Data models ---

// Park is a national park unit returned by the /parks endpoint.
type Park struct {
	ID          string `json:"id"`
	Code        string `json:"parkCode"`
	Name        string `json:"fullName"`
	Description string `json:"description"`
	States      string `json:"states"`
	URL         string `json:"url"`
	LatLong     string `json:"latLong,omitempty"`
}

// Campground is returned by the /campgrounds endpoint.
type Campground struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ParkCode        string `json:"parkCode"`
	Description     string `json:"description"`
	URL             string `json:"url,omitempty"`
	Latitude        string `json:"latitude,omitempty"`
	Longitude       string `json:"longitude,omitempty"`
	ReservableSites string `json:"numberOfSitesReservable,omitempty"`
	WalkupSites     string `json:"numberOfSitesFirstComeFirstServe,omitempty"`
}

// Alert is returned by the /alerts endpoint.
type Alert struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	ParkCode    string `json:"parkCode"`
	URL         string `json:"url,omitempty"`
}

// VisitorCenter is returned by the /visitor-centers endpoint.
type VisitorCenter struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ParkCode    string `json:"parkCode"`
	Description string `json:"description"`
	URL         string `json:"url,omitempty"`
	Latitude    string `json:"latitude,omitempty"`
	Longitude   string `json:"longitude,omitempty"`
}

// --- ParksInput and Parks ---

// ParksInput controls the /parks query.
type ParksInput struct {
	State    string // two-letter state code, e.g. "CA"
	ParkCode string // e.g. "grca"
	Limit    int    // 0 means server default (50)
	Start    int    // pagination offset
}

// Parks fetches parks from the NPS API.
func (c *Client) Parks(ctx context.Context, in ParksInput) ([]Park, error) {
	u, _ := url.Parse(c.cfg.BaseURL + "/parks")
	q := u.Query()
	if in.State != "" {
		q.Set("stateCode", in.State)
	}
	if in.ParkCode != "" {
		q.Set("parkCode", in.ParkCode)
	}
	if in.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", in.Limit))
	}
	if in.Start > 0 {
		q.Set("start", fmt.Sprintf("%d", in.Start))
	}
	u.RawQuery = q.Encode()
	body, err := c.get(ctx, u.String())
	if err != nil {
		return nil, err
	}
	var resp apiResponse[Park]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse parks: %w", err)
	}
	return resp.Data, nil
}

// --- CampgroundsInput and Campgrounds ---

// CampgroundsInput controls the /campgrounds query.
type CampgroundsInput struct {
	State    string
	ParkCode string
	Limit    int
	Start    int
}

// Campgrounds fetches campgrounds from the NPS API.
func (c *Client) Campgrounds(ctx context.Context, in CampgroundsInput) ([]Campground, error) {
	u, _ := url.Parse(c.cfg.BaseURL + "/campgrounds")
	q := u.Query()
	if in.State != "" {
		q.Set("stateCode", in.State)
	}
	if in.ParkCode != "" {
		q.Set("parkCode", in.ParkCode)
	}
	if in.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", in.Limit))
	}
	if in.Start > 0 {
		q.Set("start", fmt.Sprintf("%d", in.Start))
	}
	u.RawQuery = q.Encode()
	body, err := c.get(ctx, u.String())
	if err != nil {
		return nil, err
	}
	var resp apiResponse[Campground]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse campgrounds: %w", err)
	}
	return resp.Data, nil
}

// --- AlertsInput and Alerts ---

// AlertsInput controls the /alerts query.
type AlertsInput struct {
	ParkCode string
	Limit    int
	Start    int
}

// Alerts fetches alerts from the NPS API.
func (c *Client) Alerts(ctx context.Context, in AlertsInput) ([]Alert, error) {
	u, _ := url.Parse(c.cfg.BaseURL + "/alerts")
	q := u.Query()
	if in.ParkCode != "" {
		q.Set("parkCode", in.ParkCode)
	}
	if in.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", in.Limit))
	}
	if in.Start > 0 {
		q.Set("start", fmt.Sprintf("%d", in.Start))
	}
	u.RawQuery = q.Encode()
	body, err := c.get(ctx, u.String())
	if err != nil {
		return nil, err
	}
	var resp apiResponse[Alert]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse alerts: %w", err)
	}
	return resp.Data, nil
}

// --- VisitorCentersInput and VisitorCenters ---

// VisitorCentersInput controls the /visitor-centers query.
type VisitorCentersInput struct {
	State    string
	ParkCode string
	Limit    int
	Start    int
}

// VisitorCenters fetches visitor centers from the NPS API.
func (c *Client) VisitorCenters(ctx context.Context, in VisitorCentersInput) ([]VisitorCenter, error) {
	u, _ := url.Parse(c.cfg.BaseURL + "/visitor-centers")
	q := u.Query()
	if in.State != "" {
		q.Set("stateCode", in.State)
	}
	if in.ParkCode != "" {
		q.Set("parkCode", in.ParkCode)
	}
	if in.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", in.Limit))
	}
	if in.Start > 0 {
		q.Set("start", fmt.Sprintf("%d", in.Start))
	}
	u.RawQuery = q.Encode()
	body, err := c.get(ctx, u.String())
	if err != nil {
		return nil, err
	}
	var resp apiResponse[VisitorCenter]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse visitor-centers: %w", err)
	}
	return resp.Data, nil
}
