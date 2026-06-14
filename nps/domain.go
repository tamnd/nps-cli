package nps

import (
	"context"
	"net/url"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes the NPS API as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/nps-cli/nps"
//
// The same Domain also builds the standalone nps binary (see cli.NewApp),
// so the binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the nps driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "nps",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "nps",
			Short:  "A command line for the National Park Service.",
			Long: `A command line for the National Park Service (NPS) public API.

nps reads public NPS data over plain HTTPS, shapes it into
clean records, and prints output that pipes into the rest of your tools.
Uses the NPS Data API (developer.nps.gov). Register a free key at:
https://www.nps.gov/subjects/digital/nps-data-api.htm`,
			Site: "https://" + Host,
			Repo: "https://github.com/tamnd/nps-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "parks",
		Group:   "read",
		List:    true,
		Summary: "List national parks, filtered by state or park code",
	}, listParks)

	kit.Handle(app, kit.OpMeta{
		Name:    "campgrounds",
		Group:   "read",
		List:    true,
		Summary: "List campgrounds, filtered by state or park code",
	}, listCampgrounds)

	kit.Handle(app, kit.OpMeta{
		Name:    "alerts",
		Group:   "read",
		List:    true,
		Summary: "List park alerts",
	}, listAlerts)

	kit.Handle(app, kit.OpMeta{
		Name:    "visitor-centers",
		Group:   "read",
		List:    true,
		Summary: "List visitor centers, filtered by state or park code",
	}, listVisitorCenters)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type parksIn struct {
	State string `kit:"flag" help:"two-letter state code (e.g. CA)"`
	Code  string `kit:"flag" help:"park code (e.g. grca)"`
	Limit int    `kit:"flag,inherit" help:"max results"`
	Key   string `kit:"flag" help:"NPS API key" default:"DEMO_KEY"`

	Client *Client `kit:"inject"`
}

type campgroundsIn struct {
	State string `kit:"flag" help:"two-letter state code (e.g. CA)"`
	Park  string `kit:"flag" help:"park code (e.g. yose)"`
	Limit int    `kit:"flag,inherit" help:"max results"`
	Key   string `kit:"flag" help:"NPS API key" default:"DEMO_KEY"`

	Client *Client `kit:"inject"`
}

type alertsIn struct {
	Park  string `kit:"flag" help:"park code (e.g. yell)"`
	Limit int    `kit:"flag,inherit" help:"max results"`
	Key   string `kit:"flag" help:"NPS API key" default:"DEMO_KEY"`

	Client *Client `kit:"inject"`
}

type visitorCentersIn struct {
	State string `kit:"flag" help:"two-letter state code (e.g. CA)"`
	Park  string `kit:"flag" help:"park code (e.g. yose)"`
	Limit int    `kit:"flag,inherit" help:"max results"`
	Key   string `kit:"flag" help:"NPS API key" default:"DEMO_KEY"`

	Client *Client `kit:"inject"`
}

// --- handlers ---

func listParks(ctx context.Context, in parksIn, emit func(*Park) error) error {
	in.Client.cfg.APIKey = in.Key
	parks, err := in.Client.Parks(ctx, ParksInput{
		State:    in.State,
		ParkCode: in.Code,
		Limit:    in.Limit,
	})
	if err != nil {
		return err
	}
	for i := range parks {
		if err := emit(&parks[i]); err != nil {
			return err
		}
	}
	return nil
}

func listCampgrounds(ctx context.Context, in campgroundsIn, emit func(*Campground) error) error {
	in.Client.cfg.APIKey = in.Key
	camps, err := in.Client.Campgrounds(ctx, CampgroundsInput{
		State:    in.State,
		ParkCode: in.Park,
		Limit:    in.Limit,
	})
	if err != nil {
		return err
	}
	for i := range camps {
		if err := emit(&camps[i]); err != nil {
			return err
		}
	}
	return nil
}

func listAlerts(ctx context.Context, in alertsIn, emit func(*Alert) error) error {
	in.Client.cfg.APIKey = in.Key
	alerts, err := in.Client.Alerts(ctx, AlertsInput{
		ParkCode: in.Park,
		Limit:    in.Limit,
	})
	if err != nil {
		return err
	}
	for i := range alerts {
		if err := emit(&alerts[i]); err != nil {
			return err
		}
	}
	return nil
}

func listVisitorCenters(ctx context.Context, in visitorCentersIn, emit func(*VisitorCenter) error) error {
	in.Client.cfg.APIKey = in.Key
	vcs, err := in.Client.VisitorCenters(ctx, VisitorCentersInput{
		State:    in.State,
		ParkCode: in.Park,
		Limit:    in.Limit,
	})
	if err != nil {
		return err
	}
	for i := range vcs {
		if err := emit(&vcs[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver: the URI-native string functions, pure and network-free ---

// Classify turns any accepted input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if u, e := url.Parse(input); e == nil && (u.Scheme == "http" || u.Scheme == "https") {
		// treat the path as a page id for generic linking
		id = strings.Trim(u.Path, "/")
	} else {
		id = strings.Trim(input, "/")
	}
	if id == "" {
		return "", "", errs.Usage("unrecognized nps reference: %q", input)
	}
	return "park", id, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "park":
		return "https://www.nps.gov/" + strings.Trim(id, "/"), nil
	default:
		return "", errs.Usage("nps has no resource type %q", uriType)
	}
}
