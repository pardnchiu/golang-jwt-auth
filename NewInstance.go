package golangJwtAuth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

func New(c *Config) (*JWTAuth, error) {
	if c == nil {
		return nil, fmt.Errorf("Config is required")
	}

	if c.LogPath == "" {
		c.LogPath = "./logs/golangJWTAuth"
	}

	logger, err := NewLogger(c.LogPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to init logger: %v", err)
	}

	if c.PrivateKeyPath != "" {
		privateKeyBytes, err := os.ReadFile(c.PrivateKeyPath)
		if err != nil {
			logger.Init(true, "Private key not exists", err.Error())
			return nil, fmt.Errorf("Private key not exists: %v", err)
		}
		c.PrivateKey = string(privateKeyBytes)
	} else if c.PrivateKey == "" {
		logger.Init(true, "Private key is required")
		return nil, fmt.Errorf("Private key is required")
	}

	privateKey, err := jwt.ParseECPrivateKeyFromPEM([]byte(c.PrivateKey))
	if err != nil {
		logger.Init(true, "Invalid private key", err.Error())
		return nil, fmt.Errorf("Invalid private key: %v", err)
	}

	c.PrivateKeyPEM = privateKey

	if c.PublicKeyPath != "" {
		publicKeyBytes, err := os.ReadFile(c.PublicKeyPath)
		if err != nil {
			logger.Init(true, "Public key not exists", err.Error())
			return nil, fmt.Errorf("Public key not exists: %v", err)
		}
		c.PublicKey = string(publicKeyBytes)
	} else if c.PublicKey == "" {
		logger.Init(true, "Public key is required")
		return nil, fmt.Errorf("Public key is required")
	}

	publicKey, err := jwt.ParseECPublicKeyFromPEM([]byte(c.PublicKey))
	if err != nil {
		logger.Init(true, "Invalid public key", err.Error())
		return nil, fmt.Errorf("Invalid public key: %v", err)
	}

	c.PublicKeyPEM = publicKey

	if !c.PrivateKeyPEM.PublicKey.Equal(c.PublicKeyPEM) {
		logger.Init(true, "Private/Public mismatch")
		return nil, fmt.Errorf("Private/Public mismatch")
	}

	if c.AccessTokenExpires == 0 {
		c.AccessTokenExpires = 15 * time.Minute
	}

	if c.RefreshIdExpires == 0 {
		c.RefreshIdExpires = 7 * 24 * time.Hour
	}

	if c.Domain == "" {
		c.Domain = "localhost"
	}

	if c.AccessTokenCookieKey == "" {
		c.AccessTokenCookieKey = "access_token"
	}

	if c.RefreshIdCookieKey == "" {
		c.RefreshIdCookieKey = "refresh_id"
	}

	if c.MaxVersion == 0 {
		c.MaxVersion = 5
	}

	if c.RefreshTTL == 0 {
		c.RefreshTTL = 0.5
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port),
		Password: c.Redis.Password,
		DB:       c.Redis.DB,
	})

	context := context.Background()

	if _, err := redisClient.Ping(context).Result(); err != nil {
		logger.Init(true, "Failed to connect Redis", err.Error())
		return nil, fmt.Errorf("Failed to connect Redis: %v", err)
	}

	logger.Init(false,
		"golangJwtAuth initialized successfully",
		fmt.Sprintf("AccessTokenExpires: %s", c.AccessTokenExpires),
		fmt.Sprintf("RefreshIdExpires: %s", c.RefreshIdExpires),
		fmt.Sprintf("IsProd: %t", c.IsProd),
		fmt.Sprintf("Domain: %s", c.Domain),
		fmt.Sprintf("Redis host: %s", c.Redis.Host),
		fmt.Sprintf("Redis port: %d", c.Redis.Port),
		fmt.Sprintf("Redis db: %d", c.Redis.DB),
		fmt.Sprintf("AccessTokenCookieKey: %s", c.AccessTokenCookieKey),
		fmt.Sprintf("RefreshIdCookieKey: %s", c.RefreshIdCookieKey),
		fmt.Sprintf("MaxVersion: %d", c.MaxVersion),
		fmt.Sprintf("RefreshTTL: %f", c.RefreshTTL),
		fmt.Sprintf("LogPath: %s", c.LogPath),
	)

	return &JWTAuth{
		Config:  c,
		Redis:   redisClient,
		Context: context,
		Logger:  logger,
	}, nil
}

func (j *JWTAuth) Close() error {
	return j.Redis.Close()
}
