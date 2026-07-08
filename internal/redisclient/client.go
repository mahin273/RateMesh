package redisclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

// Client wraps the UniversalClient from go-redis/v9 to support both clustered and standalone modes.
type Client struct {
	redis.UniversalClient
}

// NewClient parses the connection address string and returns a connected Client.
// If multiple comma-separated addresses are provided, it configures a Cluster client.
func NewClient(addr string) (*Client, error) {
	var rdb redis.UniversalClient

	addrs := strings.Split(addr, ",")
	if len(addrs) > 1 {
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: addrs,
		})
	} else {
		rdb = redis.NewClient(&redis.Options{
			Addr: addr,
		})
	}

	// Test connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &Client{UniversalClient: rdb}, nil
}
