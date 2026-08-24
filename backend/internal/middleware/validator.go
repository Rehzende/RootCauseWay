package middleware

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var slugRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-]{1,98}[a-zA-Z0-9]$`)

// ValidateUUID validates that s is a valid UUID string.
func ValidateUUID(s string) error {
	if _, err := uuid.Parse(s); err != nil {
		return fmt.Errorf("invalid UUID: %s", s)
	}
	return nil
}

// ValidateSlug validates that s is a valid slug (alphanumeric + hyphens, 3-100 chars).
func ValidateSlug(s string) error {
	if len(s) < 3 || len(s) > 100 {
		return fmt.Errorf("slug must be between 3 and 100 characters")
	}
	if !slugRegexp.MatchString(s) {
		return fmt.Errorf("slug must contain only alphanumeric characters and hyphens")
	}
	return nil
}

// ValidateEmail validates that s is a valid email address.
func ValidateEmail(s string) error {
	if _, err := mail.ParseAddress(s); err != nil {
		return fmt.Errorf("invalid email address")
	}
	return nil
}

// SanitizeString trims whitespace and limits length to 10000 characters.
func SanitizeString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 10000 {
		s = s[:10000]
	}
	return s
}

// ValidateJSONPayload checks that the data length does not exceed maxBytes.
func ValidateJSONPayload(data []byte, maxBytes int64) error {
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("payload too large: %d bytes exceeds maximum of %d bytes", len(data), maxBytes)
	}
	return nil
}

// ValidatePagination clamps page and perPage to reasonable limits.
// page >= 1, perPage between 1 and 100.
func ValidatePagination(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}
