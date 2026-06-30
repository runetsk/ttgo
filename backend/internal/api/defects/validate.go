package defects

import (
	"fmt"
	"net/url"
	"strings"
)

var validSeverity = map[string]bool{"critical": true, "major": true, "minor": true, "trivial": true}
var validStatus = map[string]bool{"open": true, "closed": true}

// ValidExternalURL accepts "" or an absolute http/https URL (length-capped). XSS guard.
func ValidExternalURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if len(raw) > 2048 {
		return fmt.Errorf("external_url too long")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("external_url is not a valid URL")
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("external_url must be an absolute http or https URL")
	}
	return nil
}
