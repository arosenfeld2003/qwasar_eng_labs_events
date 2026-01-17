package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"marry-me/internal/broker"
	"marry-me/internal/config"
	"marry-me/internal/coordinator"
	"marry-me/internal/event"
	"marry-me/internal/logger"
	"marry-me/internal/organizer"
	"marry-me/internal/stress"
	"marry-me/internal/team"
)

var (
	reportFile = flag.String("report", "", "Output file for stress report (JSON)")
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup logging
	logger.Setup(cfg)

	log.Info().
		Str("dataset", cfg.DatasetPath).
		Int("workers_per_team", cfg.WorkersPerTeam).
		Msg("Marry-Me Wedding Event Simulator starting...")

	// Load dataset
	dataset, err := loadDataset(cfg.DatasetPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load dataset")
	}
	log.Info().
		Int("events", len(dataset.Events)).
		Msg("Dataset loaded")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create broker
	rmq, err := broker.NewRabbitMQ(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to RabbitMQ")
	}
	defer rmq.Close()

	// Setup queues
	if err := rmq.SetupQueues(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to setup queues")
	}

	// Create components
	coord := coordinator.New(rmq, cfg)
	org := organizer.New(rmq, cfg)
	tracker := stress.New(rmq, cfg)

	// Create teams
	teams := make(map[event.TeamName]*team.Team)
	for _, teamName := range event.AllTeams() {
		t := team.New(teamName, rmq, cfg)
		teams[teamName] = t
	}

	// Start components
	if err := tracker.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start stress tracker")
	}

	for _, t := range teams {
		if err := t.Start(ctx); err != nil {
			log.Fatal().Err(err).Str("team", string(t.Name())).Msg("Failed to start team")
		}
	}

	if err := org.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start organizer")
	}

	if err := coord.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start coordinator")
	}

	log.Info().Msg("All components started, beginning simulation...")

	// Run simulation in a goroutine
	simulationDone := make(chan struct{})
	go func() {
		defer close(simulationDone)
		runSimulation(ctx, coord, dataset, cfg)
	}()

	// Wait for completion or signal
	select {
	case <-sigCh:
		log.Info().Msg("Received shutdown signal")
		cancel()
	case <-simulationDone:
		log.Info().Msg("Simulation completed")
		// Give time for final events to process
		time.Sleep(5 * time.Second)
	}

	// Graceful shutdown
	log.Info().Msg("Shutting down...")

	coord.Stop()
	org.Stop()
	for _, t := range teams {
		t.Stop()
	}
	tracker.Stop()

	// Print and save report
	tracker.PrintReport()

	if *reportFile != "" {
		if err := tracker.WriteReportToFile(*reportFile); err != nil {
			log.Error().Err(err).Msg("Failed to write report file")
		}
	}

	log.Info().Msg("Shutdown complete")
}

func loadDataset(path string) (*event.Dataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read dataset file: %w", err)
	}

	dataset, err := event.ParseDataset(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dataset: %w", err)
	}

	return dataset, nil
}

func runSimulation(ctx context.Context, coord *coordinator.Coordinator, dataset *event.Dataset, cfg *config.Config) {
	if len(dataset.Events) == 0 {
		log.Warn().Msg("No events in dataset")
		return
	}

	// Group events by timestamp (second)
	eventsBySecond := make(map[int64][]*event.Event)
	var minTimestamp, maxTimestamp int64

	for i := range dataset.Events {
		e := &dataset.Events[i]
		ts := e.Timestamp

		if minTimestamp == 0 || ts < minTimestamp {
			minTimestamp = ts
		}
		if ts > maxTimestamp {
			maxTimestamp = ts
		}

		eventsBySecond[ts] = append(eventsBySecond[ts], e)
	}

	log.Info().
		Int64("min_timestamp", minTimestamp).
		Int64("max_timestamp", maxTimestamp).
		Int64("duration_seconds", maxTimestamp-minTimestamp+1).
		Msg("Simulation timeline")

	// Create ticker for simulation
	tickDuration := time.Duration(float64(time.Second) / cfg.SimulationSpeed)
	ticker := time.NewTicker(tickDuration)
	defer ticker.Stop()

	currentTimestamp := minTimestamp
	totalInjected := 0

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Simulation cancelled")
			return

		case <-ticker.C:
			// Inject events for current timestamp
			if events, ok := eventsBySecond[currentTimestamp]; ok {
				for _, e := range events {
					eventCopy := e.Clone()
					if err := coord.InjectEvent(ctx, eventCopy); err != nil {
						log.Error().Err(err).Str("event_id", e.ID).Msg("Failed to inject event")
					} else {
						totalInjected++
					}
				}

				log.Debug().
					Int64("timestamp", currentTimestamp).
					Int("events", len(events)).
					Int("total_injected", totalInjected).
					Msg("Events injected")
			}

			currentTimestamp++

			// Check if simulation is complete
			if currentTimestamp > maxTimestamp {
				log.Info().
					Int("total_events", totalInjected).
					Msg("All events injected")
				return
			}
		}
	}
}
