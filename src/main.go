package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

type Config struct {
	SourceEndpoint      string
	SourceAccessKey     string
	SourceSecretKey     string
	SourceBucket        string
	DestEndpoint        string
	DestAccessKey       string
	DestSecretKey       string
	DestBucket          string
	DestPrefix          string
	DryRun              bool
	MaxDelete           int
	Retries             int
	BandwidthLimit      string
	LogLevel            string
}

func loadConfig() (*Config, error) {
	sourceBucket := getEnvOrDefault("SOURCE_BUCKET", "")
	config := &Config{
		SourceEndpoint:  getEnvOrDefault("SOURCE_S3_ENDPOINT", ""),
		SourceAccessKey: getEnvOrDefault("SOURCE_ACCESS_KEY", ""),
		SourceSecretKey: getEnvOrDefault("SOURCE_SECRET_KEY", ""),
		SourceBucket:    sourceBucket,
		DestEndpoint:    getEnvOrDefault("DEST_S3_ENDPOINT", ""),
		DestAccessKey:   getEnvOrDefault("DEST_ACCESS_KEY", ""),
		DestSecretKey:   getEnvOrDefault("DEST_SECRET_KEY", ""),
		DestBucket:      getEnvOrDefault("DEST_BUCKET", ""),
		DestPrefix:      getEnvOrDefault("DEST_PREFIX", sourceBucket),
		DryRun:          getEnvOrDefault("DRY_RUN", "false") == "true",
		MaxDelete:       getEnvIntOrDefault("MAX_DELETE", 1000),
		Retries:         getEnvIntOrDefault("RETRIES", 3),
		BandwidthLimit:  getEnvOrDefault("BANDWIDTH_LIMIT", ""),
		LogLevel:        getEnvOrDefault("LOG_LEVEL", "info"),
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

func validateConfig(config *Config) error {
	required := map[string]string{
		"SOURCE_S3_ENDPOINT": config.SourceEndpoint,
		"SOURCE_ACCESS_KEY":  config.SourceAccessKey,
		"SOURCE_SECRET_KEY":  config.SourceSecretKey,
		"SOURCE_BUCKET":      config.SourceBucket,
		"DEST_S3_ENDPOINT":   config.DestEndpoint,
		"DEST_ACCESS_KEY":    config.DestAccessKey,
		"DEST_SECRET_KEY":    config.DestSecretKey,
		"DEST_BUCKET":        config.DestBucket,
	}

	for key, value := range required {
		if value == "" {
			return fmt.Errorf("required environment variable %s is not set", key)
		}
	}

	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}


func createRcloneConfig(config *Config) (string, error) {
	configDir := "/tmp/rclone-config"
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create rclone config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "rclone.conf")
	configContent := fmt.Sprintf(`[source]
type = s3
provider = Other
access_key_id = %s
secret_access_key = %s
endpoint = %s
acl = private

[dest]
type = s3
provider = Other
access_key_id = %s
secret_access_key = %s
endpoint = %s
acl = private
`, config.SourceAccessKey, config.SourceSecretKey, config.SourceEndpoint,
		config.DestAccessKey, config.DestSecretKey, config.DestEndpoint)

	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		return "", fmt.Errorf("failed to write rclone config: %w", err)
	}

	return configFile, nil
}

func runSync(config *Config, logger *logrus.Logger) error {
	configFile, err := createRcloneConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create rclone config: %w", err)
	}
	defer os.Remove(configFile)

	sourceRemote := fmt.Sprintf("source:%s", config.SourceBucket)
	destRemote := fmt.Sprintf("dest:%s/%s", config.DestBucket, config.DestPrefix)

	args := []string{
		"sync",
		sourceRemote,
		destRemote,
		"--config", configFile,
		"--delete-during",
		"--checksum",
		"--retries", strconv.Itoa(config.Retries),
		"--stats", "1m",
		"--stats-log-level", "INFO",
		"--progress",
	}

	if config.DryRun {
		args = append(args, "--dry-run")
		logger.Info("Running in dry-run mode - no changes will be made")
	}

	if config.MaxDelete > 0 {
		args = append(args, "--max-delete", strconv.Itoa(config.MaxDelete))
	}

	if config.BandwidthLimit != "" {
		args = append(args, "--bwlimit", config.BandwidthLimit)
	}

	logger.WithFields(logrus.Fields{
		"source": sourceRemote,
		"dest":   destRemote,
		"args":   args,
	}).Info("Starting rclone sync")

	cmd := exec.Command("rclone", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)

	logger.WithFields(logrus.Fields{
		"duration": duration,
		"success":  err == nil,
	}).Info("Sync operation completed")

	if err != nil {
		return fmt.Errorf("rclone sync failed: %w", err)
	}

	return nil
}

func setupLogger(level string) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	return logger
}

func main() {
	config, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	logger := setupLogger(config.LogLevel)

	logger.WithFields(logrus.Fields{
		"source_bucket": config.SourceBucket,
		"dest_bucket":   config.DestBucket,
		"dest_prefix":   config.DestPrefix,
		"dry_run":       config.DryRun,
	}).Info("Starting S3 sync job")

	if err := runSync(config, logger); err != nil {
		logger.WithError(err).Fatal("Sync operation failed")
	}

	logger.Info("S3 sync job completed successfully")
}