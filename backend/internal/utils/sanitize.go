package utils

import (
	"regexp"
	"strings"
	"unicode"
)

// SanitizeString removes or replaces invalid characters for filesystem and database use
func SanitizeString(s string) string {
	// Remove control characters
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)

	// Replace path separators and other problematic characters
	replacements := map[string]string{
		"/":  "-",
		"\\": "-",
		":":  "-",
		"*":  "",
		"?":  "",
		"<":  "(",
		">":  ")",
		"|":  "-",
	}

	for old, new := range replacements {
		s = strings.ReplaceAll(s, old, new)
	}

	// Trim spaces and dots from ends
	s = strings.Trim(s, " .")

	// Limit length
	maxLen := 255
	if len(s) > maxLen {
		s = s[:maxLen]
	}

	return s
}

// SanitizeFilename creates a safe filename from a string
func SanitizeFilename(filename string) string {
	// First sanitize the string
	sanitized := SanitizeString(filename)

	// Replace spaces with underscores
	sanitized = strings.ReplaceAll(sanitized, " ", "_")

	// Remove consecutive special characters
	re := regexp.MustCompile(`[-_]+`)
	sanitized = re.ReplaceAllString(sanitized, "-")

	// Ensure it doesn't start or end with a dash
	sanitized = strings.Trim(sanitized, "-")

	// If empty after sanitization, use a default
	if sanitized == "" {
		sanitized = "unknown"
	}

	return sanitized
}

// SanitizePath creates a safe path segment from a string
func SanitizePath(path string) string {
	// First sanitize the string
	sanitized := SanitizeString(path)

	// Replace spaces with underscores
	sanitized = strings.ReplaceAll(sanitized, " ", "_")

	// Replace directory separators with dashes
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")

	// Remove consecutive special characters
	re := regexp.MustCompile(`[-_]+`)
	sanitized = re.ReplaceAllString(sanitized, "-")

	// Ensure it doesn't start or end with a dash
	sanitized = strings.Trim(sanitized, "-")

	return sanitized
}

// RemoveDiacritics removes diacritical marks from characters
func RemoveDiacritics(s string) string {
	t := make([]rune, 0, len(s))
	for _, r := range s {
		switch r {
		case 'à', 'á', 'â', 'ã', 'ä', 'å', 'ā', 'ă', 'ą':
			t = append(t, 'a')
		case 'À', 'Á', 'Â', 'Ã', 'Ä', 'Å', 'Ā', 'Ă', 'Ą', 'A':
			t = append(t, 'A')
		case 'ç', 'ć', 'č', 'ċ', 'ĉ':
			t = append(t, 'c')
		case 'Ç', 'Ć', 'Č', 'Ċ', 'Ĉ', 'C':
			t = append(t, 'C')
		case 'ð', 'ď', 'đ':
			t = append(t, 'd')
		case 'Ð', 'Ď', 'Đ', 'D':
			t = append(t, 'D')
		case 'è', 'é', 'ê', 'ë', 'ē', 'ę', 'ě', 'ė':
			t = append(t, 'e')
		case 'È', 'É', 'Ê', 'Ë', 'Ē', 'Ę', 'Ě', 'Ė', 'E':
			t = append(t, 'E')
		case 'ğ', 'ġ', 'ĝ':
			t = append(t, 'g')
		case 'Ğ', 'Ġ', 'Ĝ', 'G':
			t = append(t, 'G')
		case 'ĥ', 'ħ', 'ĩ', 'ī', 'ï', 'ı':
			t = append(t, 'h')
		case 'Ĥ', 'Ħ', 'Ĩ', 'Ï', 'I', 'İ', 'H':
			t = append(t, 'H')
		case 'ķ':
			t = append(t, 'k')
		case 'Ķ':
			t = append(t, 'K')
		case 'ł', 'ľ', 'ĺ', 'ļ', 'ŀ':
			t = append(t, 'l')
		case 'Ł', 'Ľ', 'Ĺ', 'Ļ', 'Ŀ', 'L':
			t = append(t, 'L')
		case 'ñ', 'ń', 'ň', 'ņ', 'ŋ':
			t = append(t, 'n')
		case 'Ñ', 'Ń', 'Ň', 'Ņ', 'Ŋ', 'N':
			t = append(t, 'N')
		case 'ò', 'ó', 'ô', 'õ', 'ö', 'ø', 'ō', 'ő', 'ǫ', 'ơ':
			t = append(t, 'o')
		case 'Ò', 'Ó', 'Ô', 'Õ', 'Ö', 'Ø', 'Ō', 'Ő', 'Ǫ', 'Ơ', 'O':
			t = append(t, 'O')
		case 'ř', 'ŕ', 'ŗ':
			t = append(t, 'r')
		case 'Ŕ', 'Ř', 'R':
			t = append(t, 'R')
		case 'š', 'ś', 'ş':
			t = append(t, 's')
		case 'Š', 'Ś', 'Ş', 'S':
			t = append(t, 'S')
		case 'ť', 'ţ', 'ŧ':
			t = append(t, 't')
		case 'Ť', 'Ţ', 'Ŧ', 'T':
			t = append(t, 'T')
		case 'ù', 'ú', 'û', 'ü', 'ū', 'ű', 'ů', 'ų', 'ư':
			t = append(t, 'u')
		case 'Ù', 'Ú', 'Û', 'Ü', 'Ū', 'Ű', 'Ů', 'Ų', 'Ư', 'U':
			t = append(t, 'U')
		case 'ý', 'ÿ', 'ŷ':
			t = append(t, 'y')
		case 'Ý', 'Ÿ', 'Ŷ', 'Y':
			t = append(t, 'Y')
		case 'ž', 'ż', 'ź':
			t = append(t, 'z')
		case 'Ž', 'Ż', 'Ź', 'Z':
			t = append(t, 'Z')
		case 'æ', 'œ':
			t = append(t, 'a')
			t = append(t, 'e')
		case 'Æ', 'Œ':
			t = append(t, 'A')
			t = append(t, 'E')
		case 'ß':
			t = append(t, 's')
		case 'þ':
			t = append(t, 't')
		case 'Þ':
			t = append(t, 'T')
		default:
			t = append(t, r)
		}
	}
	return string(t)
}

// TruncateString truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// CleanSearchQuery cleans a string for use in search queries
func CleanSearchQuery(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Remove diacritics
	s = RemoveDiacritics(s)

	// Remove extra whitespace
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)

	return s
}
