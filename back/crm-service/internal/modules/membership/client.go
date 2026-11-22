package membership

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var ErrNotFound = errors.New("workspace not found")

type Client struct {
	base *url.URL
	http *http.Client
}

func New(authServiceURL string) (*Client, error) {
	base, err := url.Parse(authServiceURL)
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.RawQuery != "" || base.Fragment != "" || base.User != nil {
		return nil, errors.New("AUTH_SERVICE_URL must be an HTTP or HTTPS service address")
	}
	return &Client{base: base, http: &http.Client{
		Timeout:       3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// Role asks the owner of the membership data on every request, including reads.
func (c *Client) Role(ctx context.Context, workspaceID, userID string) (string, error) {
	endpoint := c.base.JoinPath("workspaces", url.PathEscape(workspaceID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-User-Id", userID)
	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("get workspace membership: %w", err)
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusNotFound:
		return "", ErrNotFound
	case http.StatusOK:
		var workspace struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		}
		if err := json.NewDecoder(io.LimitReader(res.Body, 16<<10)).Decode(&workspace); err != nil {
			return "", fmt.Errorf("decode workspace membership: %w", err)
		}
		if workspace.ID != workspaceID || workspace.Role == "" {
			return "", errors.New("invalid workspace membership from auth")
		}
		return workspace.Role, nil
	default:
		return "", fmt.Errorf("get workspace membership: auth returned %d", res.StatusCode)
	}
}
