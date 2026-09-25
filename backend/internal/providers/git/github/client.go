package github

import (
	"bytes"
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	gitprovider "github.com/itsmangooo/Silicon/backend/internal/providers/git"
)

const maxResponseBytes = 4 << 20

type Client struct {
	AppID      int64
	PrivateKey string
	BaseURL    string
	HTTPClient *http.Client
	Now        func() time.Time
}

func (c Client) Installation(ctx context.Context, id int64) (gitprovider.Installation, error) {
	var response struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	}
	if err := c.appRequest(ctx, http.MethodGet, "/app/installations/"+strconv.FormatInt(id, 10), nil, &response); err != nil {
		return gitprovider.Installation{}, err
	}
	return gitprovider.Installation{ID: response.ID, Account: response.Account.Login}, nil
}

func (c Client) ListRepositories(ctx context.Context, installationID int64) ([]gitprovider.Repository, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	items := []gitprovider.Repository{}
	for page := 1; page <= 100; page++ {
		var response struct {
			Repositories []struct {
				ID            int64  `json:"id"`
				FullName      string `json:"full_name"`
				DefaultBranch string `json:"default_branch"`
				Private       bool   `json:"private"`
			} `json:"repositories"`
		}
		if err := c.request(ctx, http.MethodGet, "/installation/repositories?per_page=100&page="+strconv.Itoa(page), token, nil, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Repositories {
			items = append(items, gitprovider.Repository{ID: item.ID, FullName: item.FullName, DefaultBranch: item.DefaultBranch, Private: item.Private})
		}
		if len(response.Repositories) < 100 {
			break
		}
	}
	return items, nil
}

func (c Client) Archive(ctx context.Context, installationID int64, repository, revision string) (io.ReadCloser, error) {
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, errors.New("invalid GitHub repository name")
	}
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = "https://api.github.com"
	}
	path := "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/tarball/" + url.PathEscape(revision)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Silicon")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github archive request failed: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, fmt.Errorf("github archive request failed with status %d", response.StatusCode)
	}
	return response.Body, nil
}

func (c Client) installationToken(ctx context.Context, installationID int64) (string, error) {
	var response struct {
		Token string `json:"token"`
	}
	if err := c.appRequest(ctx, http.MethodPost, "/app/installations/"+strconv.FormatInt(installationID, 10)+"/access_tokens", bytes.NewReader([]byte("{}")), &response); err != nil {
		return "", err
	}
	if response.Token == "" {
		return "", errors.New("github returned an empty installation token")
	}
	return response.Token, nil
}

func (c Client) appRequest(ctx context.Context, method, path string, body io.Reader, result any) error {
	token, err := c.jwt()
	if err != nil {
		return err
	}
	return c.request(ctx, method, path, token, body, result)
}

func (c Client) request(ctx context.Context, method, path, token string, body io.Reader, result any) error {
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = "https://api.github.com"
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Silicon")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("github request failed: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("github request failed with status %d", response.StatusCode)
	}
	if result != nil && len(data) > 0 {
		return json.Unmarshal(data, result)
	}
	return nil
}

func (c Client) jwt() (string, error) {
	block, _ := pem.Decode([]byte(strings.ReplaceAll(c.PrivateKey, `\n`, "\n")))
	if block == nil {
		return "", errors.New("github app private key is not configured")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	var key *rsa.PrivateKey
	if err == nil {
		key, _ = parsed.(*rsa.PrivateKey)
	} else {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	if err != nil || key == nil {
		return "", errors.New("github app private key is invalid")
	}
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	header := rawURLJSON(map[string]any{"alg": "RS256", "typ": "JWT"})
	claims := rawURLJSON(map[string]any{"iat": now.Add(-60 * time.Second).Unix(), "exp": now.Add(9 * time.Minute).Unix(), "iss": strconv.FormatInt(c.AppID, 10)})
	signing := header + "." + claims
	digest := sha256.Sum256([]byte(signing))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func rawURLJSON(value any) string {
	data, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(data)
}

func VerifySignature(secret string, body []byte, signature string) bool {
	if secret == "" || len(signature) != 71 || !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}

func ParseRef(ref string) (string, bool) {
	const prefix = "refs/heads/"
	if !strings.HasPrefix(ref, prefix) {
		return "", false
	}
	branch, err := url.PathUnescape(strings.TrimPrefix(ref, prefix))
	return branch, err == nil && branch != ""
}
