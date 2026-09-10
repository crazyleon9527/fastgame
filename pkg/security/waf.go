package security

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var (
	blockedUserAgents = []*regexp.Regexp{
		regexp.MustCompile(`(?i)sqlmap`),
		regexp.MustCompile(`(?i)nikto`),
		regexp.MustCompile(`(?i)nmap`),
		regexp.MustCompile(`(?i)masscan`),
		regexp.MustCompile(`(?i)dirbuster`),
		regexp.MustCompile(`(?i)hydra`),
		regexp.MustCompile(`(?i)acunetix`),
	}

	attackPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(union\s+select|drop\s+table|insert\s+into)`),
		regexp.MustCompile(`(?i)(<script|javascript:|onerror=|onload=)`),
		regexp.MustCompile(`(?i)(\.\./|\.\.\\|/etc/passwd)`),
		regexp.MustCompile(`(?i)(eval\s*\(|base64_decode)`),
	}
)

type WAF struct{}

func NewWAF() *WAF {
	return &WAF{}
}

func (w *WAF) InspectRequest(r *http.Request, body string) error {
	method := strings.ToUpper(r.Method)
	if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut && method != http.MethodDelete {
		return fmt.Errorf("method not allowed")
	}

	ua := r.Header.Get("User-Agent")
	for _, pattern := range blockedUserAgents {
		if pattern.MatchString(ua) {
			return fmt.Errorf("blocked user agent")
		}
	}

	if method == http.MethodPost || method == http.MethodPut {
		ct := r.Header.Get("Content-Type")
		if ct != "" && !strings.HasPrefix(strings.ToLower(ct), "application/json") {
			return fmt.Errorf("invalid content type")
		}
	}

	target := r.URL.Path + "?" + r.URL.RawQuery
	for _, pattern := range attackPatterns {
		if pattern.MatchString(target) {
			return fmt.Errorf("suspicious request uri")
		}
	}

	for _, pattern := range attackPatterns {
		if body != "" && pattern.MatchString(body) {
			return fmt.Errorf("suspicious request body")
		}
	}

	return nil
}
