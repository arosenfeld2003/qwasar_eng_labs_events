package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
)

// Config holds all application configuration values.
type Config struct {
	Dataset     string  // path to the JSON dataset file
	RabbitMQURL string  // AMQP connection URL
	Workers     int     // number of concurrent workers per team
	Verbose     bool    // enable verbose logging
	Speed       float64 // simulation speed multiplier
	Report      string  // output path for JSON report (empty = no report)
}

// Parse creates a Config by parsing CLI flags from the provided args.
// Environment variables DATASET_PATH and RABBITMQ_URL serve as fallbacks
// when the corresponding flags are not explicitly set.
func Parse(args []string) (*Config, error) {
	fs := flag.NewFlagSet("marry-me", flag.ContinueOnError)

	cfg := &Config{}
	fs.StringVar(&cfg.Dataset, "dataset", "datasets/dataset_1.json", "path to event dataset JSON file")
	fs.StringVar(&cfg.RabbitMQURL, "rabbitmq-url", "amqp://guest:guest@localhost:5672/", "RabbitMQ connection URL")
	fs.IntVar(&cfg.Workers, "workers", 3, "number of workers per team")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "enable verbose logging")
	fs.Float64Var(&cfg.Speed, "speed", 1.0, "simulation speed multiplier")
	fs.StringVar(&cfg.Report, "report", "", "output path for JSON report")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Track which flags were explicitly provided on the command line.
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	// Environment variable fallback: only apply when the flag was not set.
	if !explicit["dataset"] {
		if v := os.Getenv("DATASET_PATH"); v != "" {
			cfg.Dataset = v
		}
	}
	if !explicit["rabbitmq-url"] {
		if v := os.Getenv("RABBITMQ_URL"); v != "" {
			cfg.RabbitMQURL = v
		}
	}

	return cfg, nil
}

// Validate checks that all config values are acceptable.
func (c *Config) Validate() error {
	if c.Workers <= 0 {
		return fmt.Errorf("workers must be > 0, got %d", c.Workers)
	}
	if c.Speed <= 0 {
		return fmt.Errorf("speed must be > 0, got %g", c.Speed)
	}
	if _, err := os.Stat(c.Dataset); err != nil {
		return fmt.Errorf("dataset file: %w", err)
	}
	u, err := url.Parse(c.RabbitMQURL)
	if err != nil {
		return fmt.Errorf("invalid rabbitmq-url: %w", err)
	}
	if u.Scheme != "amqp" && u.Scheme != "amqps" {
		return fmt.Errorf("invalid rabbitmq-url: scheme must be amqp or amqps, got %q", u.Scheme)
	}
	return nil
}
