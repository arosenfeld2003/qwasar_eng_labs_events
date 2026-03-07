package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/config"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/coordinator"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/organizer"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/stress"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/team"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("parse config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	log.Printf("Marry-Me Wedding Event Simulator")
	log.Printf("  Dataset:  %s", cfg.Dataset)
	log.Printf("  Workers:  %d per team", cfg.Workers)
	log.Printf("  Speed:    %.1fx", cfg.Speed)
	log.Printf("  RabbitMQ: %s", cfg.RabbitMQURL)

	// Load events from dataset
	events, err := event.LoadFromFile(cfg.Dataset)
	if err != nil {
		log.Fatalf("load dataset: %v", err)
	}
	log.Printf("  Events:   %d loaded", len(events))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to RabbitMQ
	b, err := broker.NewRabbitMQ(ctx, broker.RabbitMQConfig{URL: cfg.RabbitMQURL})
	if err != nil {
		log.Fatalf("connect to RabbitMQ: %v", err)
	}
	defer b.Close()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 1. Setup coordinator
	coord := coordinator.New(b)
	if err := coord.Setup(ctx); err != nil {
		log.Fatalf("coordinator setup: %v", err)
	}

	// 2. Setup organizer (declares exchanges, team queues, bindings)
	org := organizer.New(b)
	if err := org.Setup(ctx); err != nil {
		log.Fatalf("organizer setup: %v", err)
	}

	// 3. Setup team manager (creates workers, subscribes to team queues)
	mgr := team.New(b, cfg.Workers, cfg.Speed)
	if err := mgr.Setup(ctx); err != nil {
		log.Fatalf("team manager setup: %v", err)
	}

	// 4. Setup stress tracker
	tracker := stress.New()

	// Start the organizer (routes validated events to team queues)
	go func() {
		if err := org.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("organizer error: %v", err)
		}
	}()

	// Start the stress tracker (consumes results)
	go func() {
		if err := tracker.Consume(ctx, b, organizer.QueueResults); err != nil && err != context.Canceled {
			log.Printf("tracker error: %v", err)
		}
	}()

	simDuration := time.Duration(float64(360*time.Second) / cfg.Speed)
	log.Printf("Pipeline ready. Simulating 6-hour wedding over %v (speed %.1fx)...", simDuration.Round(time.Second), cfg.Speed)
	log.Println("(Ctrl+C to stop early and see partial report)")

	// 5. Ingest events in the background; coordinator schedules each one by its
	//    wedding timestamp so they arrive at the right moment in the simulation.
	go func() {
		if err := coord.Ingest(ctx, events, cfg.Speed); err != nil && err != context.Canceled {
			log.Printf("ingest error: %v", err)
			return
		}
		s := coord.Stats()
		log.Printf("All events dispatched: %d accepted, %d rejected", s.Accepted, s.Rejected)
	}()

	// Run until the simulation window closes or the user interrupts.
	select {
	case <-sigCh:
		log.Println("Interrupted. Shutting down...")
	case <-time.After(simDuration):
		log.Println("Simulation complete.")
	}

	cancel()
	_ = mgr.Shutdown(context.Background())

	// Give tracker a moment to finalize
	time.Sleep(500 * time.Millisecond)

	// Print report
	report := tracker.Report()
	printReport(report)

	// Save report to file if configured
	if cfg.Report != "" {
		data, err := report.JSON()
		if err != nil {
			log.Printf("marshal report: %v", err)
		} else if err := os.WriteFile(cfg.Report, data, 0644); err != nil {
			log.Printf("write report: %v", err)
		} else {
			log.Printf("Report saved to %s", cfg.Report)
		}
	}
}

func printReport(r *stress.Report) {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("     WEDDING STRESS REPORT")
	fmt.Println("========================================")
	fmt.Printf("  Total Events:    %d\n", r.TotalEvents)
	fmt.Printf("  Completed:       %d\n", r.Completed)
	fmt.Printf("  Expired:         %d\n", r.Expired)
	fmt.Printf("  Overall Stress:  %.1f%%\n", r.OverallStress*100)
	fmt.Println()

	if len(r.ByPriority) > 0 {
		fmt.Println("  By Priority:")
		for _, p := range []string{"High", "Medium", "Low"} {
			if ps, ok := r.ByPriority[p]; ok {
				fmt.Printf("    %-8s %3d total | %3d completed | %3d expired | stress %.1f%%\n",
					p, ps.Total, ps.Completed, ps.Expired, ps.Stress*100)
			}
		}
		fmt.Println()
	}

	if len(r.ByTeam) > 0 {
		fmt.Println("  By Team:")
		for team, ts := range r.ByTeam {
			fmt.Printf("    %-14s %3d total | %3d completed | %3d expired | stress %.1f%%\n",
				team, ts.Total, ts.Completed, ts.Expired, ts.Stress*100)
		}
	}
	fmt.Println("========================================")
}
