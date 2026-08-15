package machine

import (
	"fmt"
	"strings"
	"unicode"
)

const maxResourceIDLength = 64

func ValidateResourceID(kind, value string) error {
	if value == "" {
		return fmt.Errorf("%w: %s required", ErrBadArgs, kind)
	}
	if len(value) > maxResourceIDLength {
		return fmt.Errorf("%w: %s must be at most %d bytes", ErrBadArgs, kind, maxResourceIDLength)
	}
	if strings.Contains(value, "..") {
		return fmt.Errorf("%w: %s must not contain path traversal", ErrBadArgs, kind)
	}
	if strings.ContainsAny(value, "?#") {
		return fmt.Errorf("%w: %s must not contain query or fragment separators", ErrBadArgs, kind)
	}
	if strings.Contains(value, "%") {
		return fmt.Errorf("%w: %s must not contain percent-encoded segments", ErrBadArgs, kind)
	}
	for i, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: %s must not contain control characters", ErrBadArgs, kind)
		}
		if !isResourceIDRune(r) {
			return fmt.Errorf("%w: %s contains unsupported character %q", ErrBadArgs, kind, r)
		}
		if i == 0 && !isASCIIAlphaNumeric(r) {
			return fmt.Errorf("%w: %s must start with an ASCII letter or digit", ErrBadArgs, kind)
		}
	}
	return nil
}

func isResourceIDRune(r rune) bool {
	return isASCIIAlphaNumeric(r) || r == '-' || r == '_' || r == '.'
}

func isASCIIAlphaNumeric(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}
