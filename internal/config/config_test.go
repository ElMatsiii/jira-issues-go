package config_test

import (
	"os"
	"testing"

	"jira-issues-go/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestConfig_LoadDefaults(t *testing.T) {
	os.Unsetenv("PORT")
	os.Unsetenv("GRPC_PORT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("KAFKA_BROKERS")
	os.Unsetenv("KAFKA_TOPIC")
	os.Unsetenv("KAFKA_TOPIC_ISSUES")
	os.Unsetenv("MIGRATIONS_DIR")

	cfg := config.LoadConfig()
	assert.Equal(t, "50051", cfg.GRPCPort)
	assert.Contains(t, cfg.DatabaseURL, "localhost:5432/issues_db")
	assert.Equal(t, []string{"localhost:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, "issues.events", cfg.KafkaTopic)
	assert.Equal(t, "db/migrations", cfg.MigrationsDir)
}

func TestConfig_CustomEnv(t *testing.T) {
	t.Setenv("PORT", "50099")
	t.Setenv("DATABASE_URL", "postgres://custom:pass@pg-host:5432/test_db?sslmode=require")
	t.Setenv("KAFKA_BROKERS", "kafka1:9092, kafka2:9092")
	t.Setenv("KAFKA_TOPIC", "custom.events")
	t.Setenv("MIGRATIONS_DIR", "/custom/migrations")

	cfg := config.LoadConfig()
	assert.Equal(t, "50099", cfg.GRPCPort)
	assert.Equal(t, "postgres://custom:pass@pg-host:5432/test_db?sslmode=require", cfg.DatabaseURL)
	assert.Equal(t, []string{"kafka1:9092", "kafka2:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, "custom.events", cfg.KafkaTopic)
	assert.Equal(t, "/custom/migrations", cfg.MigrationsDir)
}

func TestConfig_FallbackEnvNames(t *testing.T) {
	os.Unsetenv("PORT")
	t.Setenv("GRPC_PORT", "50055")
	os.Unsetenv("KAFKA_TOPIC")
	t.Setenv("KAFKA_TOPIC_ISSUES", "fallback.topic")

	cfg := config.LoadConfig()
	assert.Equal(t, "50055", cfg.GRPCPort)
	assert.Equal(t, "fallback.topic", cfg.KafkaTopic)
}
