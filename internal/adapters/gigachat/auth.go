package gigachat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type oauthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.accessToken != "" && time.Now().Before(c.accessTokenExpiresAt.Add(-1*time.Minute)) {
		t := c.accessToken
		c.mu.Unlock()
		return t, nil
	}
	c.mu.Unlock()

	if c.cfg.AuthorizationKey == "" {
		return "", errors.New("gigachat: GIGACHAT_AUTHORIZATION_KEY is empty")
	}
	authKey := strings.TrimSpace(c.cfg.AuthorizationKey)
	authKey = strings.TrimPrefix(authKey, "Basic ")
	authKey = strings.TrimPrefix(authKey, "basic ")

	form := url.Values{}
	form.Set("scope", strings.TrimSpace(c.cfg.Scope))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.AuthURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("gigachat: build auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RqUID", newRqUID())
	req.Header.Set("Authorization", "Basic "+authKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gigachat: auth request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("gigachat: auth bad status: %s, body: %s", res.Status, strings.TrimSpace(string(body)))
	}

	var oauth oauthResponse
	if err := json.NewDecoder(res.Body).Decode(&oauth); err != nil {
		return "", fmt.Errorf("gigachat: decode auth response: %w", err)
	}
	if oauth.AccessToken == "" {
		return "", errors.New("gigachat: access_token is empty in auth response")
	}

	exp := time.Now().Add(30 * time.Minute)
	if oauth.ExpiresAt > 0 {
		exp = time.UnixMilli(oauth.ExpiresAt)
	}

	c.mu.Lock()
	c.accessToken = oauth.AccessToken
	c.accessTokenExpiresAt = exp
	c.mu.Unlock()

	return oauth.AccessToken, nil
}

func (c *Client) authorizedRequest(ctx context.Context, method, rawURL string, body []byte, contentType string) (*http.Request, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gigachat: build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("RqUID", newRqUID())
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return req, nil
}
