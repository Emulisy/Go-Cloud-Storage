package cache

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

//MP upload initial info
type MPUploadInfo struct {
	FileHash   string `json:"fileHash"`
	FileSize   int64  `json:"fileSize"`
	UploadID   string `json:"uploadId"`
	ChunkSize  int64  `json:"chunkSize"`
	ChunkCount int64  `json:"chunkCount"`
}

var client *redis.Client

// InitRedis creates the shared Redis client and verifies the connection.
func InitRedis() error {
	addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:6379"
	}

	database := 0
	if rawDatabase := strings.TrimSpace(os.Getenv("REDIS_DB")); rawDatabase != "" {
		parsedDatabase, err := strconv.Atoi(rawDatabase)
		if err != nil || parsedDatabase < 0 {
			return fmt.Errorf("REDIS_DB must be a non-negative integer")
		}
		database = parsedDatabase
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       database,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close()
		return fmt.Errorf("connect to Redis at %s: %w", addr, err)
	}

	client = redisClient
	return nil
}

// RedisClient returns the shared Redis client.
func RedisClient() *redis.Client {
	return client
}

// CloseRedis closes the shared Redis client.
func CloseRedis() error {
	if client == nil {
		return nil
	}

	err := client.Close()
	client = nil
	return err
}
