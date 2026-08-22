package sheets

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrCredentials = errors.New("Google Sheets credentials are unavailable")
	ErrUnavailable = errors.New("Google Sheets API is unavailable")
)

type Snapshot struct {
	Revision string
	Values   [][]string
}

type Source interface {
	Fetch(context.Context, Config) (Snapshot, error)
}

type GoogleClientConfig struct {
	APIBase         string
	APIKey          string
	AccessToken     string
	CredentialsFile string
	MaxBytes        int64
	Timeout         time.Duration
}

type GoogleClient struct {
	http   *http.Client
	config GoogleClientConfig
	tokens *serviceAccountTokens
}

func NewGoogleClient(client *http.Client, config GoogleClientConfig) *GoogleClient {
	if client == nil {
		client = &http.Client{}
	}
	if config.APIBase == "" {
		config.APIBase = "https://sheets.googleapis.com"
	}
	if config.MaxBytes <= 0 {
		config.MaxBytes = 4 << 20
	}
	if config.Timeout <= 0 {
		config.Timeout = 20 * time.Second
	}
	return &GoogleClient{http: client, config: config, tokens: &serviceAccountTokens{}}
}

func (c *GoogleClient) Fetch(ctx context.Context, config Config) (Snapshot, error) {
	return c.fetch(ctx, config, "")
}

// FetchWithAccessToken reads a Sheet selected through a user's Google Drive
// connection. The token is supplied by the encrypted OAuth connection store.
func (c *GoogleClient) FetchWithAccessToken(ctx context.Context, config Config, accessToken string) (Snapshot, error) {
	return c.fetch(ctx, config, strings.TrimSpace(accessToken))
}

func (c *GoogleClient) fetch(ctx context.Context, config Config, tokenOverride string) (Snapshot, error) {
	base, err := url.Parse(c.config.APIBase)
	if err != nil || base.Scheme != "https" || base.Hostname() == "" || base.User != nil {
		return Snapshot{}, fmt.Errorf("%w: invalid API endpoint", ErrUnavailable)
	}
	rangeValue := config.SheetName + "!" + config.Range
	pathPrefix := strings.TrimRight(base.Path, "/")
	escapedPrefix := strings.TrimRight(base.EscapedPath(), "/")
	base.Path = pathPrefix + "/v4/spreadsheets/" + config.SpreadsheetID + "/values/" + rangeValue
	base.RawPath = escapedPrefix + "/v4/spreadsheets/" + url.PathEscape(config.SpreadsheetID) + "/values/" + url.PathEscape(rangeValue)
	query := base.Query()
	query.Set("majorDimension", "ROWS")
	query.Set("valueRenderOption", "UNFORMATTED_VALUE")
	if c.config.APIKey != "" {
		query.Set("key", c.config.APIKey)
	}
	base.RawQuery = query.Encode()
	requestCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, base.String(), nil)
	if err != nil {
		return Snapshot{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "DomainMonitor/phase5")
	accessToken := tokenOverride
	if accessToken == "" {
		accessToken = strings.TrimSpace(c.config.AccessToken)
	}
	if accessToken == "" && c.config.CredentialsFile != "" {
		accessToken, err = c.tokens.token(requestCtx, c.http, c.config.CredentialsFile)
		if err != nil {
			return Snapshot{}, err
		}
	}
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if accessToken == "" && c.config.APIKey == "" {
		return Snapshot{}, ErrCredentials
	}
	response, err := c.http.Do(request)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, c.config.MaxBytes+1))
	if err != nil || int64(len(body)) > c.config.MaxBytes {
		return Snapshot{}, fmt.Errorf("%w: response too large or unreadable", ErrUnavailable)
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return Snapshot{}, ErrCredentials
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Snapshot{}, fmt.Errorf("%w: HTTP %d", ErrUnavailable, response.StatusCode)
	}
	var payload struct {
		Values [][]any `json:"values"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return Snapshot{}, fmt.Errorf("%w: invalid response JSON", ErrUnavailable)
	}
	values := make([][]string, len(payload.Values))
	for rowIndex, row := range payload.Values {
		values[rowIndex] = make([]string, len(row))
		for columnIndex, value := range row {
			values[rowIndex][columnIndex] = cellString(value)
		}
	}
	revision := strings.TrimSpace(response.Header.Get("ETag"))
	if revision == "" {
		revision = strings.TrimSpace(response.Header.Get("Last-Modified"))
	}
	return Snapshot{Revision: revision, Values: values}, nil
}

func cellString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		return typed.String()
	default:
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	}
}

type serviceAccountCredentials struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type serviceAccountTokens struct {
	mu          sync.Mutex
	file        string
	cachedToken string
	expires     time.Time
}

func (c *serviceAccountTokens) token(ctx context.Context, client *http.Client, filename string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UTC()
	if c.file == filename && c.cachedToken != "" && c.expires.After(now.Add(time.Minute)) {
		return c.cachedToken, nil
	}
	raw, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("%w: read service account", ErrCredentials)
	}
	var credentials serviceAccountCredentials
	if json.Unmarshal(raw, &credentials) != nil || credentials.ClientEmail == "" || credentials.PrivateKey == "" {
		return "", fmt.Errorf("%w: invalid service account JSON", ErrCredentials)
	}
	if credentials.TokenURI == "" {
		credentials.TokenURI = "https://oauth2.googleapis.com/token"
	}
	tokenURL, err := url.Parse(credentials.TokenURI)
	if err != nil || tokenURL.Scheme != "https" || tokenURL.Hostname() != "oauth2.googleapis.com" {
		return "", fmt.Errorf("%w: unsafe token URI", ErrCredentials)
	}
	block, _ := pem.Decode([]byte(credentials.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("%w: invalid private key", ErrCredentials)
	}
	var privateKey *rsa.PrivateKey
	if parsed, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes); parseErr == nil {
		privateKey, _ = parsed.(*rsa.PrivateKey)
	} else if parsed, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes); parseErr == nil {
		privateKey = parsed
	}
	if privateKey == nil {
		return "", fmt.Errorf("%w: unsupported private key", ErrCredentials)
	}
	assertion, err := signedAssertion(privateKey, credentials.ClientEmail, credentials.TokenURI, now)
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, credentials.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: token exchange", ErrCredentials)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("%w: token HTTP %d", ErrCredentials, response.StatusCode)
	}
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if json.Unmarshal(body, &tokenResponse) != nil || tokenResponse.AccessToken == "" {
		return "", fmt.Errorf("%w: invalid token response", ErrCredentials)
	}
	if tokenResponse.ExpiresIn < 120 {
		tokenResponse.ExpiresIn = 3600
	}
	c.file, c.cachedToken, c.expires = filename, tokenResponse.AccessToken, now.Add(time.Duration(tokenResponse.ExpiresIn)*time.Second)
	return c.cachedToken, nil
}

func signedAssertion(privateKey *rsa.PrivateKey, email, audience string, now time.Time) (string, error) {
	encode := base64.RawURLEncoding.EncodeToString
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{"iss": email, "scope": "https://www.googleapis.com/auth/spreadsheets.readonly", "aud": audience, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
	unsigned := encode(header) + "." + encode(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + encode(signature), nil
}
