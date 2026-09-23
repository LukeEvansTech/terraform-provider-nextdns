package nextdns

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matryer/is"
)

// extendedSecurityKeys are the security switches the NextDNS API exposes but
// upstream nextdns-go v0.5.0 did not model: ten added in v0.3.0, and
// newlyActiveDomains, which the API began returning in September 2026.
var extendedSecurityKeys = []string{
	"freeHostingDomains",
	"tunnelingEndpoints",
	"dataDropServices",
	"residentialHosting",
	"untrustedCertificates",
	"fastFluxNetworks",
	"dnsDataExfiltration",
	"dnsPayloadDelivery",
	"decentralizedWebGateways",
	"highRiskTlds",
	"newlyActiveDomains",
}

// capturePatch returns a test server that records the JSON body of the last
// PATCH it received, and a pointer to the captured map.
func capturePatch(t *testing.T) (*httptest.Server, *map[string]any) {
	t.Helper()
	captured := map[string]any{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			captured = map[string]any{}
			if err := json.Unmarshal(body, &captured); err != nil {
				t.Fatal(err)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)
	return ts, &captured
}

func TestSecurityUpdateOmitsUnsetExtendedFields(t *testing.T) {
	c := is.New(t)
	ts, captured := capturePatch(t)
	client, err := New(WithBaseURL(ts.URL))
	c.NoErr(err)

	err = client.Security.Update(context.Background(), &UpdateSecurityRequest{
		ProfileID: "abc123",
		Security:  &Security{ThreatIntelligenceFeeds: true},
	})
	c.NoErr(err)

	// The legacy switches are always sent (upstream behaviour, unchanged).
	_, ok := (*captured)["threatIntelligenceFeeds"]
	c.True(ok)
	// None of the extended switches appear when left nil.
	for _, k := range extendedSecurityKeys {
		_, ok := (*captured)[k]
		c.True(!ok) // key must be absent
	}
}

func TestSecurityUpdateSendsExplicitExtendedValues(t *testing.T) {
	c := is.New(t)
	ts, captured := capturePatch(t)
	client, err := New(WithBaseURL(ts.URL))
	c.NoErr(err)

	err = client.Security.Update(context.Background(), &UpdateSecurityRequest{
		ProfileID: "abc123",
		Security: &Security{
			FreeHostingDomains:       Bool(false),
			TunnelingEndpoints:       Bool(true),
			DataDropServices:         Bool(false),
			ResidentialHosting:       Bool(false),
			UntrustedCertificates:    Bool(false),
			FastFluxNetworks:         Bool(false),
			DNSDataExfiltration:      Bool(false),
			DNSPayloadDelivery:       Bool(false),
			DecentralizedWebGateways: Bool(false),
			HighRiskTlds:             Bool(false),
			NewlyActiveDomains:       Bool(false),
		},
	})
	c.NoErr(err)

	// An explicit false must be serialised, not dropped by omitempty.
	c.Equal((*captured)["freeHostingDomains"], false)
	c.Equal((*captured)["tunnelingEndpoints"], true)
	for _, k := range extendedSecurityKeys {
		_, ok := (*captured)[k]
		c.True(ok)
	}
}

func TestSecurityGetExtendedFieldsPresent(t *testing.T) {
	c := is.New(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"threatIntelligenceFeeds":true,"freeHostingDomains":false,"tunnelingEndpoints":true,"highRiskTlds":false}}`))
	}))
	t.Cleanup(ts.Close)
	client, err := New(WithBaseURL(ts.URL))
	c.NoErr(err)

	sec, err := client.Security.Get(context.Background(), &GetSecurityRequest{ProfileID: "abc123"})
	c.NoErr(err)
	c.True(sec.FreeHostingDomains != nil)
	c.Equal(*sec.FreeHostingDomains, false)
	c.True(sec.TunnelingEndpoints != nil)
	c.Equal(*sec.TunnelingEndpoints, true)
	c.True(sec.HighRiskTlds != nil)
	c.Equal(*sec.HighRiskTlds, false)
	// Not in the response: must stay nil, not become false.
	c.True(sec.FastFluxNetworks == nil)
	c.True(sec.DNSDataExfiltration == nil)
}

func TestSecurityGetExtendedFieldsAbsent(t *testing.T) {
	c := is.New(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"threatIntelligenceFeeds":true,"csam":true}}`))
	}))
	t.Cleanup(ts.Close)
	client, err := New(WithBaseURL(ts.URL))
	c.NoErr(err)

	sec, err := client.Security.Get(context.Background(), &GetSecurityRequest{ProfileID: "abc123"})
	c.NoErr(err)
	for _, p := range []*bool{
		sec.FreeHostingDomains, sec.TunnelingEndpoints, sec.DataDropServices, sec.ResidentialHosting,
		sec.UntrustedCertificates, sec.FastFluxNetworks, sec.DNSDataExfiltration, sec.DNSPayloadDelivery,
		sec.DecentralizedWebGateways, sec.HighRiskTlds, sec.NewlyActiveDomains,
	} {
		c.True(p == nil)
	}
}

func TestSettingsUpdateOmitsUnsetBav(t *testing.T) {
	c := is.New(t)
	ts, captured := capturePatch(t)
	client, err := New(WithBaseURL(ts.URL))
	c.NoErr(err)

	err = client.Settings.Update(context.Background(), &UpdateSettingsRequest{
		ProfileID: "abc123",
		Settings:  &Settings{Web3: true},
	})
	c.NoErr(err)
	_, ok := (*captured)["web3"]
	c.True(ok)
	_, ok = (*captured)["bav"]
	c.True(!ok)
}

func TestSettingsUpdateSendsExplicitBav(t *testing.T) {
	c := is.New(t)
	ts, captured := capturePatch(t)
	client, err := New(WithBaseURL(ts.URL))
	c.NoErr(err)

	for _, want := range []bool{true, false} {
		err = client.Settings.Update(context.Background(), &UpdateSettingsRequest{
			ProfileID: "abc123",
			Settings:  &Settings{Bav: Bool(want)},
		})
		c.NoErr(err)
		c.Equal((*captured)["bav"], want)
	}
}

func TestSettingsGetBav(t *testing.T) {
	c := is.New(t)
	bodies := []string{
		`{"data":{"web3":true,"bav":true}}`,
		`{"data":{"web3":true}}`,
	}
	i := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(bodies[i]))
		i++
	}))
	t.Cleanup(ts.Close)
	client, err := New(WithBaseURL(ts.URL))
	c.NoErr(err)

	s, err := client.Settings.Get(context.Background(), &GetSettingsRequest{ProfileID: "abc123"})
	c.NoErr(err)
	c.True(s.Bav != nil)
	c.Equal(*s.Bav, true)

	s, err = client.Settings.Get(context.Background(), &GetSettingsRequest{ProfileID: "abc123"})
	c.NoErr(err)
	c.True(s.Bav == nil)
}
