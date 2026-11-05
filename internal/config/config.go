package config

import (
	"os"
	"strings"
)

// Config encapsulates server runtime settings loaded from environment variables.
type Config struct {
	GRPCPort      string
	DatabaseURL   string
	KafkaBrokers  []string
	KafkaTopic    string
	MigrationsDir string
}

// LoadConfig loads application configuration from environment with defaults.
func LoadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("GRPC_PORT")
	}
	if port == "" {
		port = "50051"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/issues_db?sslmode=disable"
	}

	brokersStr := os.Getenv("KAFKA_BROKERS")
	if brokersStr == "" {
		brokersStr = "localhost:9092"
	}
	rawBrokers := strings.Split(brokersStr, ",")
	brokers := make([]string, 0, len(rawBrokers))
	for _, b := range rawBrokers {
		trimmed := strings.TrimSpace(b)
		if trimmed != "" {
			brokers = append(brokers, trimmed)
		}
	}
	if len(brokers) == 0 {
		brokers = []string{"localhost:9092"}
	}

	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		topic = os.Getenv("KAFKA_TOPIC_ISSUES")
	}
	if topic == "" {
		topic = "issues.events"
	}

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "db/migrations"
	}

	return &Config{
		GRPCPort:      port,
		DatabaseURL:   dbURL,
		KafkaBrokers:  brokers,
		KafkaTopic:    topic,
		MigrationsDir: migrationsDir,
	}
}
