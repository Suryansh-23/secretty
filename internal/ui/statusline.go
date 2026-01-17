package ui

import (
	"fmt"

	"github.com/suryansh-23/secretty/internal/types"
)

// StatusLine formats a minimal status line for redaction events.
func StatusLine(count int, strict bool, includeID bool, secretType types.SecretType, id int) string {
	if count <= 0 {
		return ""
	}
	prefix := "secretty:"
	if strict {
		prefix = "secretty(strict):"
	}
	if count > 1 {
		return fmt.Sprintf("%s redacted %d secrets", prefix, count)
	}
	if includeID && id > 0 {
		return fmt.Sprintf("%s redacted %s#%d", prefix, secretType, id)
	}
	return fmt.Sprintf("%s redacted %s", prefix, secretType)
}
