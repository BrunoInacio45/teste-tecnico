package config

import "os"

type Config struct {
	AWSEndpoint          string
	AWSRegion            string
	ProcessedEventsQueue string
	EventsTable          string
	SummaryTable         string
	Port                 string
}

func Load() Config {
	return Config{
		AWSEndpoint:          getEnv("AWS_ENDPOINT", "http://localhost:4566"),
		AWSRegion:            getEnv("AWS_REGION", "us-east-1"),
		ProcessedEventsQueue: getEnv("PROCESSED_EVENTS_QUEUE", "processed-events"),
		EventsTable:          getEnv("EVENTS_TABLE", "events"),
		SummaryTable:         getEnv("SUMMARY_TABLE", "developer_summary"),
		Port:                 getEnv("PORT", "8080"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
