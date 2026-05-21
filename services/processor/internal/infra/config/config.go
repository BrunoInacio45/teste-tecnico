package config

import (
	"os"
	"strconv"
)

type Config struct {
	AWSEndpoint          string
	AWSRegion            string
	RawEventsQueue       string
	ProcessedEventsQueue string
	WorkersCount         int
	ProcessorID          string
}

func Load() Config {
	return Config{
		AWSEndpoint:          getEnv("AWS_ENDPOINT", "http://localhost:4566"),
		AWSRegion:            getEnv("AWS_REGION", "us-east-1"),
		RawEventsQueue:       getEnv("RAW_EVENTS_QUEUE", "raw-events"),
		ProcessedEventsQueue: getEnv("PROCESSED_EVENTS_QUEUE", "processed-events"),
		WorkersCount:         getEnvInt("WORKERS_COUNT", 5),
		ProcessorID:          getEnv("PROCESSOR_ID", "processor-1"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}
