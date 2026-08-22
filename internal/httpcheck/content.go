package httpcheck

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"domainmonitor/internal/netcheck"
)

var (
	titlePattern   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	tagPattern     = regexp.MustCompile(`(?s)<[^>]*>`)
	secretPattern  = regexp.MustCompile(`(?i)(authorization|api[_-]?key|token|password)([\s"'=:]+)([^\s"'<>;&]{4,})`)
	htmlWhitespace = regexp.MustCompile(`\s+`)
)

func readContent(response *http.Response, mode ContentMode, config Config) ContentEvidence {
	evidence := ContentEvidence{
		Status:                "UNKNOWN",
		ContentType:           response.Header.Get("Content-Type"),
		DeclaredContentLength: response.ContentLength,
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, config.MaxBodyBytes+1))
	if err != nil {
		evidence.ErrorCode = netcheck.HTTPBodyRead
		evidence.ErrorMessage = "read response body: " + err.Error()
		return evidence
	}
	evidence.BodySize = int64(len(payload))
	oversized := int64(len(payload)) > config.MaxBodyBytes
	if oversized {
		payload = payload[:config.MaxBodyBytes]
	}
	hash := sha256.Sum256(payload)
	evidence.BodySHA256 = hash[:]
	evidence.HashComplete = !oversized
	excerptLength := min(len(payload), config.ExcerptBytes)
	if excerptLength > 0 && isTextual(evidence.ContentType, payload) {
		evidence.Excerpt = redactExcerpt(payload[:excerptLength])
	}
	evidence.Title = extractTitle(payload)

	switch {
	case oversized:
		evidence.Status = "OVERSIZED_TRUNCATED"
		evidence.ErrorCode = netcheck.ContentTooLarge
		evidence.ErrorMessage = fmt.Sprintf("decoded response body exceeds %d bytes", config.MaxBodyBytes)
		return evidence
	case mode == ContentStatusOnly && len(payload) == 0:
		evidence.Status = "EMPTY"
		return evidence
	case mode == ContentStatusOnly:
		if looksLikeHTML(evidence.ContentType, payload) {
			evidence.Status = "VALID_HTML"
		} else {
			evidence.Status = "VALID_NON_HTML"
		}
		return evidence
	case len(payload) == 0:
		evidence.Status = "EMPTY"
		evidence.ErrorCode = netcheck.ContentEmpty
		evidence.ErrorMessage = "response body is empty"
		return evidence
	case len(payload) < config.MinMeaningfulBytes:
		evidence.Status = "TOO_SMALL"
		evidence.ErrorCode = netcheck.ContentTooSmall
		evidence.ErrorMessage = fmt.Sprintf("response body is smaller than %d bytes", config.MinMeaningfulBytes)
		return evidence
	}

	html := looksLikeHTML(evidence.ContentType, payload)
	if mode == ContentHTML && !html {
		evidence.Status = "UNSUPPORTED_CONTENT"
		evidence.ErrorCode = netcheck.ContentUnsupported
		evidence.ErrorMessage = "response is not HTML"
		return evidence
	}
	if html && meaningfulText(payload) < max(1, config.MinMeaningfulBytes/4) {
		evidence.Status = "NOT_MEANINGFUL"
		evidence.ErrorCode = netcheck.ContentNotMeaningful
		evidence.ErrorMessage = "HTML response does not contain meaningful text"
		return evidence
	}
	if html {
		evidence.Status = "VALID_HTML"
	} else {
		evidence.Status = "VALID_NON_HTML"
	}
	return evidence
}

func looksLikeHTML(contentType string, payload []byte) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	mediaType = strings.ToLower(mediaType)
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		return true
	}
	prefix := strings.ToLower(string(payload[:min(len(payload), 1024)]))
	return strings.Contains(prefix, "<!doctype html") || strings.Contains(prefix, "<html") || strings.Contains(prefix, "<body")
}

func isTextual(contentType string, payload []byte) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if strings.HasPrefix(strings.ToLower(mediaType), "text/") || strings.Contains(strings.ToLower(mediaType), "json") || looksLikeHTML(contentType, payload) {
		return true
	}
	return utf8.Valid(payload)
}

func meaningfulText(payload []byte) int {
	withoutTags := tagPattern.ReplaceAllString(string(payload), " ")
	normalized := htmlWhitespace.ReplaceAllString(strings.TrimSpace(withoutTags), " ")
	count := 0
	for _, value := range normalized {
		if unicode.IsLetter(value) || unicode.IsNumber(value) {
			count++
		}
	}
	return count
}

func extractTitle(payload []byte) string {
	matches := titlePattern.FindSubmatch(payload)
	if len(matches) != 2 {
		return ""
	}
	title := htmlWhitespace.ReplaceAllString(strings.TrimSpace(tagPattern.ReplaceAllString(string(matches[1]), " ")), " ")
	runes := []rune(title)
	if len(runes) > 512 {
		title = string(runes[:512])
	}
	return title
}

func redactExcerpt(payload []byte) []byte {
	redacted := secretPattern.ReplaceAll(payload, []byte("$1$2[REDACTED]"))
	return append([]byte(nil), redacted...)
}

var selectedHeaderNames = []string{
	"Content-Type", "Content-Length", "Server", "Cache-Control", "Age", "ETag", "Last-Modified",
	"Via", "X-Cache", "Cf-Ray", "Cf-Cache-Status", "X-Request-Id",
}

func selectHeaders(header http.Header, maxBytes int) map[string]string {
	selected := make(map[string]string)
	remaining := maxBytes
	for _, name := range selectedHeaderNames {
		value := strings.Join(header.Values(name), ", ")
		if value == "" || remaining <= len(name)+2 {
			continue
		}
		allowed := remaining - len(name) - 2
		if len(value) > allowed {
			value = value[:allowed]
		}
		selected[name] = value
		remaining -= len(name) + len(value) + 2
	}
	return selected
}
