package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host               string
	TokenID            string
	TokenSecret        string
	VerifySSL          bool
	CACertPath         string
	Fingerprint        string
	BindAddress        string
	Port               string
	MCPAuthToken       string
	AllowedOrigins     []string
	Timeout            time.Duration
	LogLevel           string
	EnablePower        bool
	EnableConfig       bool
	EnableStorage      bool
	EnableDestroy      bool
	EnableNodeOps      bool
	EnableNodeShutdown bool
	AllowedNodes       []string
}

func parseBoolEnv(key string) bool {
	if val, ok := os.LookupEnv(key); ok {
		lowerVal := strings.ToLower(strings.TrimSpace(val))
		return lowerVal == "true" || lowerVal == "1" || lowerVal == "yes" || lowerVal == "on"
	}
	return false
}

func LoadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		eqIdx := strings.Index(trimmed, "=")
		if eqIdx == -1 {
			continue
		}
		key := strings.TrimSpace(trimmed[:eqIdx])
		val := strings.TrimSpace(trimmed[eqIdx+1:])
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

func LoadFromEnv() (*Config, error) {
	host := strings.TrimSpace(os.Getenv("PVE_HOST"))
	tokenID := strings.TrimSpace(os.Getenv("PVE_TOKEN_ID"))
	tokenSecret := strings.TrimSpace(os.Getenv("PVE_TOKEN_SECRET"))

	if host == "" {
		return nil, errors.New("PVE_HOST is required")
	}
	if tokenID == "" {
		return nil, errors.New("PVE_TOKEN_ID is required")
	}
	if tokenSecret == "" {
		return nil, errors.New("PVE_TOKEN_SECRET is required")
	}

	parsedURL, err := url.ParseRequestURI(host)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, errors.New("PVE_HOST must be a valid http or https URL")
	}

	cleanHost := strings.TrimRight(parsedURL.Scheme+"://"+parsedURL.Host+parsedURL.Path, "/")

	verifySSL := true
	if val, ok := os.LookupEnv("PVE_VERIFY_SSL"); ok {
		lowerVal := strings.ToLower(strings.TrimSpace(val))
		if lowerVal == "false" || lowerVal == "0" || lowerVal == "no" || lowerVal == "off" {
			verifySSL = false
		}
	}

	bindAddress := strings.TrimSpace(os.Getenv("MCP_BIND_ADDRESS"))
	if bindAddress == "" {
		bindAddress = "127.0.0.1"
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	} else {
		port = strings.TrimPrefix(port, ":")
	}

	mcpAuthToken := strings.TrimSpace(os.Getenv("MCP_AUTH_TOKEN"))

	var allowedOrigins []string
	if origins := strings.TrimSpace(os.Getenv("MCP_ALLOWED_ORIGINS")); origins != "" {
		for _, o := range strings.Split(origins, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}

	timeout := 30 * time.Second
	if timeoutSecStr := strings.TrimSpace(os.Getenv("PVE_TIMEOUT_SECONDS")); timeoutSecStr != "" {
		sec, err := strconv.Atoi(timeoutSecStr)
		if err != nil || sec <= 0 {
			return nil, errors.New("PVE_TIMEOUT_SECONDS must be a positive integer")
		}
		timeout = time.Duration(sec) * time.Second
	}

	logLevel := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
	if logLevel == "" {
		logLevel = "info"
	}

	caCertPath := strings.TrimSpace(os.Getenv("PVE_CA_CERT"))

	fingerprint := strings.ToLower(strings.TrimSpace(os.Getenv("PVE_FINGERPRINT")))
	fingerprint = strings.ReplaceAll(fingerprint, ":", "")
	fingerprint = strings.ReplaceAll(fingerprint, " ", "")

	enablePower := parseBoolEnv("PVE_ENABLE_POWER")
	enableConfig := parseBoolEnv("PVE_ENABLE_CONFIG")
	enableStorage := parseBoolEnv("PVE_ENABLE_STORAGE")
	enableDestroy := parseBoolEnv("PVE_ENABLE_DESTROY")
	enableNodeOps := parseBoolEnv("PVE_ENABLE_NODE_OPS")
	enableNodeShutdown := parseBoolEnv("PVE_ENABLE_NODE_SHUTDOWN")

	var allowedNodes []string
	if nodesStr := strings.TrimSpace(os.Getenv("PVE_ALLOWED_NODES")); nodesStr != "" {
		for _, n := range strings.Split(nodesStr, ",") {
			trimmed := strings.TrimSpace(n)
			if trimmed != "" {
				allowedNodes = append(allowedNodes, trimmed)
			}
		}
	}

	return &Config{
		Host:               cleanHost,
		TokenID:            tokenID,
		TokenSecret:        tokenSecret,
		VerifySSL:          verifySSL,
		CACertPath:         caCertPath,
		Fingerprint:        fingerprint,
		BindAddress:        bindAddress,
		Port:               port,
		MCPAuthToken:       mcpAuthToken,
		AllowedOrigins:     allowedOrigins,
		Timeout:            timeout,
		LogLevel:           logLevel,
		EnablePower:        enablePower,
		EnableConfig:       enableConfig,
		EnableStorage:      enableStorage,
		EnableDestroy:      enableDestroy,
		EnableNodeOps:      enableNodeOps,
		EnableNodeShutdown: enableNodeShutdown,
		AllowedNodes:       allowedNodes,
	}, nil
}
