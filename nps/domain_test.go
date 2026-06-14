package nps

import (
	"testing"
)

// These tests exercise the URI driver's pure string functions
// and the domain info. The client's HTTP behaviour is covered in nps_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "nps" {
		t.Errorf("Scheme = %q, want nps", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "nps" {
		t.Errorf("Identity.Binary = %q, want nps", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"grca", "park", "grca"},
		{"/yose/", "park", "yose"},
		{"https://www.nps.gov/yell", "park", "yell"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("park", "grca")
	want := "https://www.nps.gov/grca"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("trail", "foo")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.APIKey != "DEMO_KEY" {
		t.Errorf("APIKey = %q, want DEMO_KEY", cfg.APIKey)
	}
	if cfg.BaseURL == "" {
		t.Error("BaseURL is empty")
	}
	if cfg.Retries <= 0 {
		t.Error("Retries should be > 0")
	}
}
