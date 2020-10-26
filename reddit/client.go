package reddit

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

const redditAPIUrl = "https://oauth.reddit.com"

// Client provides access to reddit API
type Client struct {
	client    *http.Client
	tokenator *tokenator
	cfg       AppConfig
	url       *url.URL
}

// AppConfig holds credentials for reddit API
type AppConfig struct {
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	UserAgent string `yaml:"user-agent"`
	APIUrl    string `yaml:"api-url"`
	AppID     string `yaml:"id"`
	AppSecret string `yaml:"secret"`
}

// NewClient creates Client instance
func NewClient(ctx context.Context, cfg AppConfig) (*Client, error) {
	if cfg.APIUrl == "" {
		cfg.APIUrl = redditAPIUrl
	}
	apiURL, err := url.Parse(cfg.APIUrl)
	if err != nil {
		return nil, err
	}
	tokenator := &tokenator{cfg: cfg}
	err = tokenator.StartUpdater(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start token updated: %s", err)
	}
	return &Client{
		url:       apiURL,
		client:    &http.Client{},
		tokenator: tokenator,
		cfg:       cfg,
	}, nil
}

func (c *Client) Close() {
	c.tokenator.Close()
}
