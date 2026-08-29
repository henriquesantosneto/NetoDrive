package syncer

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// IsConnectionError reports network / server reachability failures.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "actively refused") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "network is unreachable") ||
		strings.Contains(s, "timeout") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "dial tcp")
}

// Ping checks that the NetoDrive server responds (fast fail when offline).
func (c *Client) Ping() error {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/api/health", nil)
	if err != nil {
		return err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("health status %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
