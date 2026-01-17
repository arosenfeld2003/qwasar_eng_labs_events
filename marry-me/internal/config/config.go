package config

import (
	"flag"
	"os"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Dataset
	DatasetPath string

	// RabbitMQ
	RabbitMQURL string

	// Queue names
	IncomingQueue  string
	ValidatedQueue string
	ResultsQueue   string

	// Exchange
	ExchangeName string

	// Team configuration
	WorkersPerTeam int

	// Worker timing
	EventHandlingTime time.Duration

	// Routine configurations
	StandardRoutine     RoutineConfig
	IntermittentRoutine RoutineConfig
	ConcentratedRoutine RoutineConfig

	// Logging
	Verbose   bool
	JSONLogs  bool
	LogLevel  string

	// Simulation
	SimulationSpeed float64
}

// RoutineConfig defines work/idle cycle for a worker routine
type RoutineConfig struct {
	WorkDuration time.Duration
	IdleDuration time.Duration
}

// QueueNames for each team
type TeamQueues struct {
	Catering    string
	Decoration  string
	Photography string
	Music       string
	Coordinator string
	Floral      string
	Venue       string
	Transport   string
}

// Default configuration values
var DefaultConfig = Config{
	DatasetPath: "datasets/dataset_1.json",

	RabbitMQURL: "amqp://guest:guest@localhost:5672/",

	IncomingQueue:  "events.incoming",
	ValidatedQueue: "events.validated",
	ResultsQueue:   "events.results",

	ExchangeName: "marry-me",

	WorkersPerTeam: 3,

	EventHandlingTime: 3 * time.Second,

	StandardRoutine: RoutineConfig{
		WorkDuration: 20 * time.Second,
		IdleDuration: 5 * time.Second,
	},
	IntermittentRoutine: RoutineConfig{
		WorkDuration: 5 * time.Second,
		IdleDuration: 5 * time.Second,
	},
	ConcentratedRoutine: RoutineConfig{
		WorkDuration: 60 * time.Second,
		IdleDuration: 60 * time.Second,
	},

	Verbose:   false,
	JSONLogs:  false,
	LogLevel:  "info",

	SimulationSpeed: 1.0,
}

// TeamQueueNames returns the queue names for each team
func TeamQueueNames() TeamQueues {
	return TeamQueues{
		Catering:    "team.catering",
		Decoration:  "team.decoration",
		Photography: "team.photography",
		Music:       "team.music",
		Coordinator: "team.coordinator",
		Floral:      "team.floral",
		Venue:       "team.venue",
		Transport:   "team.transport",
	}
}

// GetTeamQueue returns the queue name for a given team
func GetTeamQueue(team string) string {
	return "team." + team
}

// Load loads configuration from CLI flags and environment variables
func Load() *Config {
	cfg := DefaultConfig

	// CLI flags
	flag.StringVar(&cfg.DatasetPath, "dataset", cfg.DatasetPath, "Path to the dataset JSON file")
	flag.StringVar(&cfg.RabbitMQURL, "rabbitmq-url", cfg.RabbitMQURL, "RabbitMQ connection URL")
	flag.IntVar(&cfg.WorkersPerTeam, "workers", cfg.WorkersPerTeam, "Number of workers per team")
	flag.BoolVar(&cfg.Verbose, "verbose", cfg.Verbose, "Enable verbose logging")
	flag.BoolVar(&cfg.Verbose, "v", cfg.Verbose, "Enable verbose logging (shorthand)")
	flag.BoolVar(&cfg.JSONLogs, "json-logs", cfg.JSONLogs, "Output logs in JSON format")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")
	flag.Float64Var(&cfg.SimulationSpeed, "speed", cfg.SimulationSpeed, "Simulation speed multiplier")

	flag.Parse()

	// Override with environment variables if set
	if url := os.Getenv("RABBITMQ_URL"); url != "" {
		cfg.RabbitMQURL = url
	}
	if path := os.Getenv("DATASET_PATH"); path != "" {
		cfg.DatasetPath = path
	}

	return &cfg
}

// RoutineType represents the type of worker routine
type RoutineType int

const (
	RoutineStandard RoutineType = iota
	RoutineIntermittent
	RoutineConcentrated
)

// String returns the string representation of RoutineType
func (r RoutineType) String() string {
	switch r {
	case RoutineStandard:
		return "standard"
	case RoutineIntermittent:
		return "intermittent"
	case RoutineConcentrated:
		return "concentrated"
	default:
		return "unknown"
	}
}

// GetRoutineConfig returns the configuration for a routine type
func (c *Config) GetRoutineConfig(routineType RoutineType) RoutineConfig {
	switch routineType {
	case RoutineStandard:
		return c.StandardRoutine
	case RoutineIntermittent:
		return c.IntermittentRoutine
	case RoutineConcentrated:
		return c.ConcentratedRoutine
	default:
		return c.StandardRoutine
	}
}

// Deadline durations based on priority
const (
	HighPriorityDeadline   = 5 * time.Second
	MediumPriorityDeadline = 10 * time.Second
	LowPriorityDeadline    = 20 * time.Second
)
