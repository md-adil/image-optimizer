package image

import (
	"fmt"
	"os"
	"strings"
)

var allowedDomains = map[string]bool{}

// LoadWhitelistedDomains reads the comma-separated list from env and stores in a map
func LoadWhitelistedDomains() {
	domains := os.Getenv("WHITELISTED_DOMAINS")
	if domains == "" {
		println("No whitelisted domains found in environment variable 'WHITELISTED_DOMAINS'")
		return
	}
	for domain := range strings.SplitSeq(domains, ",") {
		trimmed := strings.TrimSpace(domain)
		if trimmed != "" {
			allowedDomains[trimmed] = true
		}
	}
	fmt.Printf("Loaded domains: %v\n", allowedDomains)
}

// IsDomainAllowed checks if a domain is whitelisted
func IsDomainAllowed(domain string) bool {
	return allowedDomains[domain]
}
