package nps_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/nps-cli/nps"
)

func newTestClient(ts *httptest.Server) *nps.Client {
	cfg := nps.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.APIKey = "TEST"
	return nps.NewClient(cfg)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestParksList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/parks" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("api_key") != "TEST" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeJSON(w, map[string]any{
			"total": "2",
			"limit": "2",
			"start": "0",
			"data": []map[string]any{
				{"id": "1", "parkCode": "abli", "fullName": "Abraham Lincoln Birthplace NHP", "description": "desc1", "states": "KY", "url": "https://nps.gov/abli"},
				{"id": "2", "parkCode": "acad", "fullName": "Acadia National Park", "description": "desc2", "states": "ME", "url": "https://nps.gov/acad"},
			},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	parks, err := c.Parks(context.Background(), nps.ParksInput{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(parks) != 2 {
		t.Fatalf("got %d parks, want 2", len(parks))
	}
	if parks[0].Name != "Abraham Lincoln Birthplace NHP" {
		t.Errorf("parks[0].Name = %q", parks[0].Name)
	}
	if parks[0].Code != "abli" {
		t.Errorf("parks[0].Code = %q, want abli", parks[0].Code)
	}
}

func TestParksStateFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("stateCode")
		if state != "CA" {
			t.Errorf("stateCode = %q, want CA", state)
		}
		writeJSON(w, map[string]any{
			"total": "1",
			"limit": "1",
			"start": "0",
			"data": []map[string]any{
				{"id": "3", "parkCode": "yose", "fullName": "Yosemite National Park", "description": "yosemite", "states": "CA", "url": "https://nps.gov/yose"},
			},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	parks, err := c.Parks(context.Background(), nps.ParksInput{State: "CA", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(parks) != 1 {
		t.Fatalf("got %d parks, want 1", len(parks))
	}
	if parks[0].States != "CA" {
		t.Errorf("parks[0].States = %q, want CA", parks[0].States)
	}
}

func TestCampgrounds(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/campgrounds" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{
			"total": "96",
			"limit": "2",
			"start": "0",
			"data": []map[string]any{
				{
					"id": "10", "name": "Azalea Campground", "parkCode": "seki",
					"description": "A campground in Kings Canyon",
					"latitude": "36.76", "longitude": "-118.96",
					"numberOfSitesReservable": "0",
					"numberOfSitesFirstComeFirstServe": "110",
				},
				{
					"id": "11", "name": "Manzanita Lake Campground", "parkCode": "lavo",
					"description": "Lakeside camping",
					"latitude": "40.53", "longitude": "-121.57",
					"numberOfSitesReservable": "179",
					"numberOfSitesFirstComeFirstServe": "0",
				},
			},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	camps, err := c.Campgrounds(context.Background(), nps.CampgroundsInput{State: "CA", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(camps) != 2 {
		t.Fatalf("got %d campgrounds, want 2", len(camps))
	}
	if camps[0].Name != "Azalea Campground" {
		t.Errorf("camps[0].Name = %q", camps[0].Name)
	}
	if camps[1].ReservableSites != "179" {
		t.Errorf("camps[1].ReservableSites = %q, want 179", camps[1].ReservableSites)
	}
}

func TestAlerts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/alerts" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{
			"total": "610",
			"limit": "2",
			"start": "0",
			"data": []map[string]any{
				{
					"id": "100", "title": "Road Closure", "description": "North entrance road closed",
					"category": "Park Closure", "parkCode": "yell",
					"url": "https://nps.gov/yell/alert1",
				},
				{
					"id": "101", "title": "Bear Activity", "description": "Increased bear activity in area",
					"category": "Information", "parkCode": "yose",
					"url": "https://nps.gov/yose/alert2",
				},
			},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	alerts, err := c.Alerts(context.Background(), nps.AlertsInput{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 2 {
		t.Fatalf("got %d alerts, want 2", len(alerts))
	}
	if alerts[0].Title != "Road Closure" {
		t.Errorf("alerts[0].Title = %q", alerts[0].Title)
	}
	if alerts[1].Category != "Information" {
		t.Errorf("alerts[1].Category = %q", alerts[1].Category)
	}
}

func TestRetryOn503(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, map[string]any{
			"total": "1", "limit": "1", "start": "0",
			"data": []map[string]any{
				{"id": "99", "parkCode": "test", "fullName": "Test Park", "description": "recovered", "states": "CA", "url": "https://nps.gov/test"},
			},
		})
	}))
	defer ts.Close()

	cfg := nps.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.APIKey = "TEST"
	cfg.Retries = 5
	c := nps.NewClient(cfg)

	start := time.Now()
	parks, err := c.Parks(context.Background(), nps.ParksInput{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if len(parks) != 1 {
		t.Fatalf("got %d parks after retry", len(parks))
	}
	if parks[0].Name != "Test Park" {
		t.Errorf("parks[0].Name = %q", parks[0].Name)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestAPIKeyInRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("api_key")
		if !strings.Contains(r.URL.RawQuery, "api_key=MYKEY") {
			t.Errorf("api_key not in query: got %q", key)
		}
		writeJSON(w, map[string]any{"total": "0", "limit": "0", "start": "0", "data": []any{}})
	}))
	defer ts.Close()

	cfg := nps.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.APIKey = "MYKEY"
	c := nps.NewClient(cfg)

	_, err := c.Parks(context.Background(), nps.ParksInput{})
	if err != nil {
		t.Fatal(err)
	}
}
