package config

import "os"

type Config struct {
	AWSEndpoint          string
	AWSRegion            string
	RawEventsQueue       string
	ProcessedEventsQueue string
}

func Load() Config {
	return Config{
		AWSEndpoint:          getEnv("AWS_ENDPOINT", "http://localhost:4566"),
		AWSRegion:            getEnv("AWS_REGION", "us-east-1"),
		RawEventsQueue:       getEnv("RAW_EVENTS_QUEUE", "raw-events"),
		ProcessedEventsQueue: getEnv("PROCESSED_EVENTS_QUEUE", "processed-events"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
