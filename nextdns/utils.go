package nextdns

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	// NextDNSDomain is the domain name of the NextDNS service.
	NextDNSDomain = "nextdns.io"
)

// DNSOverHTTPSAddress returns the endpoint for DNS over HTTPS for a given profile ID.
func DNSOverHTTPSAddress(profileID string) string {
	return "https://dns." + NextDNSDomain + "/" + profileID
}

// DNSOverTLSAddress returns the endpoint for DNS over TLS for a given profile ID.
func DNSOverTLSAddress(profileID string) string {
	return profileID + ".dns." + NextDNSDomain
}

// optionalBool returns the value of a top-level boolean attribute as it was
// written in the configuration: a pointer when the user set it (true or
// false), nil when it is absent or unknown. d.Get cannot make that
// distinction for TypeBool (it returns false for both), so this reads the raw
// config that Terraform sent instead. Used for the Optional+Computed switches
// whose omission must mean "leave the live value alone".
func optionalBool(d *schema.ResourceData, key string) *bool {
	raw := d.GetRawConfig()
	if raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType() || !raw.Type().HasAttribute(key) {
		return nil
	}
	v := raw.GetAttr(key)
	if v.IsNull() || !v.IsKnown() {
		return nil
	}
	b := v.True()
	return &b
}

// setBoolIfPresent writes an API boolean into state only when the API
// returned it. A field the API omits (a feature not offered on this profile)
// keeps its previous state value, so it neither resets nor drifts.
func setBoolIfPresent(d *schema.ResourceData, key string, v *bool) error {
	if v == nil {
		return nil
	}
	return d.Set(key, *v)
}
