package httputil

import "regexp"

var (
	bearerTokenPattern = regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)([^\s",]+)`)
	jsonSecretPattern  = regexp.MustCompile(`(?i)("(?:api_key|token|refresh_token|password|secret|authorization)"\s*:\s*")((?:\\.|[^"\\])*)(")`)
	querySecretPattern = regexp.MustCompile(`(?i)(\b(?:api_key|token|refresh_token|password|secret|authorization)=)([^&\s]+)`)
	kvSecretPattern    = regexp.MustCompile(`(?i)(\b(?:api_key|token|refresh_token|password|secret)\s*:\s*)([^\s,]+)`)
)

// RedactSecrets removes common secret values before logging arbitrary strings.
func RedactSecrets(input string) string {
	redacted := bearerTokenPattern.ReplaceAllString(input, `${1}[REDACTED]`)
	redacted = jsonSecretPattern.ReplaceAllString(redacted, `${1}[REDACTED]${3}`)
	redacted = querySecretPattern.ReplaceAllString(redacted, `${1}[REDACTED]`)
	redacted = kvSecretPattern.ReplaceAllString(redacted, `${1}[REDACTED]`)
	return redacted
}
