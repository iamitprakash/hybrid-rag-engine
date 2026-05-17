package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	redis *redis.Client
	ttl   time.Duration
}

func New(redisURL string, ttl time.Duration) (*Client, error) {
	if redisURL == "" || ttl <= 0 {
		return &Client{ttl: ttl}, nil
	}
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Client{redis: redis.NewClient(options), ttl: ttl}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.redis != nil && c.ttl > 0
}

func (c *Client) GetJSON(ctx context.Context, key string, out any) (bool, error) {
	if !c.Enabled() {
		return false, nil
	}
	value, err := c.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(value), out); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) SetJSON(ctx context.Context, key string, value any) error {
	if !c.Enabled() {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.redis.Set(ctx, key, payload, c.ttl).Err()
}

func Key(namespace string, payload any) string {
	body, _ := json.Marshal(payload)
	sum := sha256.Sum256(body)
	return namespace + ":" + hex.EncodeToString(sum[:])
}
