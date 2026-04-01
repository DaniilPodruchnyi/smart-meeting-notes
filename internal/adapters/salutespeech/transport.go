package salutespeech

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *Client) doAPI(ctx context.Context, method, url string, payload []byte, contentType string, out any, action string) error {
	req, err := c.authorizedRequest(ctx, method, url, payload, contentType)
	if err != nil {
		return err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("salutespeech: %s failed: %w", action, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("salutespeech: %s bad status: %s, body: %s", action, res.Status, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("salutespeech: decode %s response: %w", action, err)
	}

	return nil
}
