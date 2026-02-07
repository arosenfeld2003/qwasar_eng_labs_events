package config

import (
	"os"
	"path/filepath"
	"testing"
)

// tmpDataset creates a temporary JSON file and returns its path.
func tmpDataset(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "events.json")
	if err := os.WriteFile(p, []byte(`[]`), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseDefaults(t *testing.T) {
	cfg, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dataset != "datasets/dataset_1.json" {
		t.Errorf("Dataset = %q, want %q", cfg.Dataset, "datasets/dataset_1.json")
	}
	if cfg.RabbitMQURL != "amqp://guest:guest@localhost:5672/" {
		t.Errorf("RabbitMQURL = %q", cfg.RabbitMQURL)
	}
	if cfg.Workers != 3 {
		t.Errorf("Workers = %d, want 3", cfg.Workers)
	}
	if cfg.Verbose {
		t.Error("Verbose should default to false")
	}
	if cfg.Speed != 1.0 {
		t.Errorf("Speed = %g, want 1.0", cfg.Speed)
	}
	if cfg.Report != "" {
		t.Errorf("Report = %q, want empty", cfg.Report)
	}
}

func TestParseCLIFlags(t *testing.T) {
	args := []string{
		"--dataset", "/tmp/d.json",
		"--rabbitmq-url", "amqp://u:p@host:1234/",
		"--workers", "5",
		"--verbose",
		"--speed", "2.5",
		"--report", "out.json",
	}
	cfg, err := Parse(args)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dataset != "/tmp/d.json" {
		t.Errorf("Dataset = %q", cfg.Dataset)
	}
	if cfg.RabbitMQURL != "amqp://u:p@host:1234/" {
		t.Errorf("RabbitMQURL = %q", cfg.RabbitMQURL)
	}
	if cfg.Workers != 5 {
		t.Errorf("Workers = %d", cfg.Workers)
	}
	if !cfg.Verbose {
		t.Error("Verbose should be true")
	}
	if cfg.Speed != 2.5 {
		t.Errorf("Speed = %g", cfg.Speed)
	}
	if cfg.Report != "out.json" {
		t.Errorf("Report = %q", cfg.Report)
	}
}

func TestParseEnvVarFallback(t *testing.T) {
	t.Setenv("DATASET_PATH", "/env/data.json")
	t.Setenv("RABBITMQ_URL", "amqp://env:env@envhost:9999/")

	cfg, err := Parse(nil) // no CLI flags
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dataset != "/env/data.json" {
		t.Errorf("Dataset = %q, want env override", cfg.Dataset)
	}
	if cfg.RabbitMQURL != "amqp://env:env@envhost:9999/" {
		t.Errorf("RabbitMQURL = %q, want env override", cfg.RabbitMQURL)
	}
}

func TestParseCLIOverridesEnv(t *testing.T) {
	t.Setenv("DATASET_PATH", "/env/data.json")
	t.Setenv("RABBITMQ_URL", "amqp://env:env@envhost:9999/")

	args := []string{
		"--dataset", "/cli/data.json",
		"--rabbitmq-url", "amqp://cli:cli@clihost:1111/",
	}
	cfg, err := Parse(args)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dataset != "/cli/data.json" {
		t.Errorf("Dataset = %q, want CLI override", cfg.Dataset)
	}
	if cfg.RabbitMQURL != "amqp://cli:cli@clihost:1111/" {
		t.Errorf("RabbitMQURL = %q, want CLI override", cfg.RabbitMQURL)
	}
}

func TestValidateWorkersZero(t *testing.T) {
	cfg := &Config{Workers: 0, Speed: 1, Dataset: tmpDataset(t), RabbitMQURL: "amqp://localhost/"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for workers=0")
	}
}

func TestValidateWorkersNegative(t *testing.T) {
	cfg := &Config{Workers: -1, Speed: 1, Dataset: tmpDataset(t), RabbitMQURL: "amqp://localhost/"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for workers=-1")
	}
}

func TestValidateSpeedZero(t *testing.T) {
	cfg := &Config{Workers: 1, Speed: 0, Dataset: tmpDataset(t), RabbitMQURL: "amqp://localhost/"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for speed=0")
	}
}

func TestValidateSpeedNegative(t *testing.T) {
	cfg := &Config{Workers: 1, Speed: -0.5, Dataset: tmpDataset(t), RabbitMQURL: "amqp://localhost/"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for speed<0")
	}
}

func TestValidateDatasetNotFound(t *testing.T) {
	cfg := &Config{Workers: 1, Speed: 1, Dataset: "/no/such/file.json", RabbitMQURL: "amqp://localhost/"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for missing dataset file")
	}
}

func TestValidateOK(t *testing.T) {
	cfg := &Config{
		Workers:     2,
		Speed:       1.5,
		Dataset:     tmpDataset(t),
		RabbitMQURL: "amqp://guest:guest@localhost:5672/",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestValidateReportFlag(t *testing.T) {
	args := []string{"--report", "results.json"}
	cfg, err := Parse(args)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Report != "results.json" {
		t.Errorf("Report = %q, want results.json", cfg.Report)
	}
}
