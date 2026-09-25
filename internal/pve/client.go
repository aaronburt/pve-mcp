package pve

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/aaronburt/pve-mcp/internal/config"
)

const maxPayloadBytes = 10 * 1024 * 1024

type Client struct {
	httpClient   *http.Client
	baseURL      string
	tokenHeader  string
	tokenSecret  string
	tokenPattern *regexp.Regexp
}

func buildTLSConfig(cfg *config.Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if !cfg.VerifySSL {
		tlsConfig.InsecureSkipVerify = true
	}

	if cfg.CACertPath != "" {
		caData, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read ca cert file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caData) {
			return nil, errors.New("failed to parse ca certificate pem")
		}
		tlsConfig.RootCAs = pool
	}

	if cfg.Fingerprint != "" {
		tlsConfig.InsecureSkipVerify = true
		expectedFP := cfg.Fingerprint
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("no certificates presented by peer")
			}
			for _, certBytes := range rawCerts {
				hash := sha256.Sum256(certBytes)
				actualFP := hex.EncodeToString(hash[:])
				if strings.EqualFold(actualFP, expectedFP) {
					return nil
				}
			}
			return errors.New("tls certificate fingerprint mismatch")
		}
	}

	return tlsConfig, nil
}

func NewClient(cfg *config.Config) (*Client, error) {
	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	var clonedTransport *http.Transport
	if ok {
		clonedTransport = transport.Clone()
	} else {
		clonedTransport = &http.Transport{}
	}
	clonedTransport.TLSClientConfig = tlsConfig

	httpClient := &http.Client{
		Transport: clonedTransport,
		Timeout:   cfg.Timeout,
	}

	tokenHeader := fmt.Sprintf("PVEAPIToken=%s=%s", cfg.TokenID, cfg.TokenSecret)
	tokenPattern := regexp.MustCompile(`PVEAPIToken=[^\s"]+`)

	return &Client{
		httpClient:   httpClient,
		baseURL:      cfg.Host,
		tokenHeader:  tokenHeader,
		tokenSecret:  cfg.TokenSecret,
		tokenPattern: tokenPattern,
	}, nil
}

func (c *Client) Sanitize(text string) string {
	result := text
	if c.tokenSecret != "" {
		result = strings.ReplaceAll(result, c.tokenSecret, "[REDACTED]")
	}
	result = c.tokenPattern.ReplaceAllString(result, "PVEAPIToken=[REDACTED]")
	return result
}

func (c *Client) Get(ctx context.Context, path string, query url.Values, target any) error {
	cleanPath := path
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	if !strings.HasPrefix(cleanPath, "/api2/json") {
		cleanPath = "/api2/json" + cleanPath
	}

	fullURL := c.baseURL + cleanPath
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return errors.New(c.Sanitize(err.Error()))
	}

	req.Header.Set("Authorization", c.tokenHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.New(c.Sanitize(err.Error()))
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, maxPayloadBytes+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return errors.New(c.Sanitize(err.Error()))
	}

	if len(body) > maxPayloadBytes {
		return errors.New("response exceeds maximum allowed size (10MB)")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pve api error (status %d): %s", resp.StatusCode, c.Sanitize(string(body)))
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("failed to decode response envelope: %w", err)
	}

	if target != nil {
		if rawPtr, ok := target.(*json.RawMessage); ok {
			*rawPtr = envelope.Data
			return nil
		}
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			return fmt.Errorf("failed to decode response data: %w", err)
		}
	}

	return nil
}

func (c *Client) Post(ctx context.Context, path string, form url.Values, target any) error {
	return c.sendWithBody(ctx, http.MethodPost, path, form, target)
}

func (c *Client) Put(ctx context.Context, path string, form url.Values, target any) error {
	return c.sendWithBody(ctx, http.MethodPut, path, form, target)
}

func (c *Client) Delete(ctx context.Context, path string, form url.Values, target any) error {
	return c.sendWithBody(ctx, http.MethodDelete, path, form, target)
}

type TaskStatus struct {
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus"`
	ID         string `json:"id"`
	Type       string `json:"type"`
	User       string `json:"user"`
	StartTime  int64  `json:"starttime"`
	PID        int    `json:"pid"`
}

func (c *Client) GetTaskStatus(ctx context.Context, node string, upid string) (*TaskStatus, error) {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", url.PathEscape(node), url.PathEscape(upid))
	var taskStatus TaskStatus
	if err := c.Get(ctx, path, nil, &taskStatus); err != nil {
		return nil, err
	}
	return &taskStatus, nil
}

func (c *Client) sendWithBody(ctx context.Context, method string, path string, form url.Values, target any) error {
	cleanPath := path
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	if !strings.HasPrefix(cleanPath, "/api2/json") {
		cleanPath = "/api2/json" + cleanPath
	}

	fullURL := c.baseURL + cleanPath

	var bodyReader io.Reader
	if method == http.MethodDelete {
		if len(form) > 0 {
			fullURL += "?" + form.Encode()
		}
	} else if len(form) > 0 {
		bodyReader = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return errors.New(c.Sanitize(err.Error()))
	}

	req.Header.Set("Authorization", c.tokenHeader)
	req.Header.Set("Accept", "application/json")
	if method != http.MethodDelete && len(form) > 0 {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.New(c.Sanitize(err.Error()))
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, maxPayloadBytes+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return errors.New(c.Sanitize(err.Error()))
	}

	if len(body) > maxPayloadBytes {
		return errors.New("response exceeds maximum allowed size (10MB)")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pve api error (status %d): %s", resp.StatusCode, c.Sanitize(string(body)))
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("failed to decode response envelope: %w", err)
	}

	if target != nil {
		if rawPtr, ok := target.(*json.RawMessage); ok {
			*rawPtr = envelope.Data
			return nil
		}
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			return fmt.Errorf("failed to decode response data: %w", err)
		}
	}

	return nil
}
