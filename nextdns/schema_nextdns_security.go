package nextdns

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceNextDNSSecuritySchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"profile_id": {
			Description: "The profile identifier to target the resource.",
			Type:        schema.TypeString,
			Required:    true,
		},
		"threat_intelligence_feeds": {
			Description: "Threat intelligence feeds.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"ai_threat_detection": {
			Description: "AI-Driven threat detection.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"google_safe_browsing": {
			Description: "Google safe browsing.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"crypto_jacking": {
			Description: "Cryptojacking protection.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"dns_rebinding": {
			Description: "DNS rebinding protection.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"idn_homographs": {
			Description: "IDN homograph attacks protection.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"typo_squatting": {
			Description: "Typosquatting protection.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"dga": {
			Description: "Domain generation algorithms (DGAs) protection.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"nrd": {
			Description: "Block newly registered domains (NRDs).",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"ddns": {
			Description: "Block dynamic DNS hostnames.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"parking": {
			Description: "Block parked domains.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"csam": {
			Description: "Block child sexual abuse material.",
			Type:        schema.TypeBool,
			Required:    true,
		},
		"tlds": {
			Description: "Block top-level domains (TLDs).",
			Type:        schema.TypeList,
			Optional:    true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},

		// Extended switches (v0.3.0). Optional + Computed: omitting one keeps
		// the live value and records it in state; an explicit true/false is
		// written; one the API stops returning keeps its last known value.
		"free_hosting_domains":       extendedSecuritySwitch("Block domains on free hosting providers (API: `freeHostingDomains`)."),
		"tunneling_endpoints":        extendedSecuritySwitch("Block tunneling and DNS-over-anything endpoints (API: `tunnelingEndpoints`)."),
		"data_drop_services":         extendedSecuritySwitch("Block data drop / paste services (API: `dataDropServices`)."),
		"residential_hosting":        extendedSecuritySwitch("Block domains served from residential IP space (API: `residentialHosting`)."),
		"untrusted_certificates":     extendedSecuritySwitch("Block domains presenting untrusted certificates (API: `untrustedCertificates`)."),
		"fast_flux_networks":         extendedSecuritySwitch("Block fast-flux networks (API: `fastFluxNetworks`). Hidden in the dashboard for some profiles; the API returns it, but enforcement is unverified."),
		"dns_data_exfiltration":      extendedSecuritySwitch("Block DNS data exfiltration (API: `dnsDataExfiltration`). Hidden in the dashboard for some profiles; the API returns it, but enforcement is unverified."),
		"dns_payload_delivery":       extendedSecuritySwitch("Block DNS payload delivery (API: `dnsPayloadDelivery`)."),
		"decentralized_web_gateways": extendedSecuritySwitch("Block decentralized web (IPFS, ENS) gateways (API: `decentralizedWebGateways`)."),
		"high_risk_tlds":             extendedSecuritySwitch("Block high-risk TLDs (API: `highRiskTlds`)."),
	}
}

// extendedSecuritySwitch is the shared schema for the v0.3.0 security switches.
func extendedSecuritySwitch(description string) *schema.Schema {
	return &schema.Schema{
		Description: description,
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	}
}
