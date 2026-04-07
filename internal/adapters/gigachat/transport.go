package gigachat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *Client) doJSON(ctx context.Context, method, url string, payload []byte, out any, action string) error {
	req, err := c.authorizedRequest(ctx, method, url, payload, "application/json")
	if err != nil {
		return err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gigachat: %s failed: %w", action, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("gigachat: read %s response: %w", action, err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("gigachat: %s bad status: %s, body: %s", action, res.Status, strings.TrimSpace(string(body)))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("gigachat: decode %s response: %w", action, err)
	}

	return nil
}
