// demo-watcher: a live CLI monitor for the Marry-Me wedding event pipeline.
//
// It polls the RabbitMQ Management HTTP API (port 15672) every 500ms and
// redraws a dashboard showing each pipeline stage, queue depths, message
// rates, and the specific Go code responsible for each stage.
//
// Usage:
//
//	./bin/demo-watcher --speed=5.0 --workers=3
//
// Run this in a second terminal while marry-me is running.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// ── ANSI helpers ─────────────────────────────────────────────────────────────

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	cyan   = "\033[36m"
	white  = "\033[97m"

	clearScreen  = "\033[2J"
	cursorHome   = "\033[H"
	hideCursor   = "\033[?25l"
	showCursor   = "\033[?25h"
)

func clr(color, s string) string { return color + s + reset }
func b(s string) string          { return bold + s + reset }
func d(s string) string          { return dim + s + reset }

// ── RabbitMQ Management API types ────────────────────────────────────────────

type apiQueue struct {
	Name         string        `json:"name"`
	Messages     int           `json:"messages"`
	Consumers    int           `json:"consumers"`
	MessageStats *messageStats `json:"message_stats"`
}

type messageStats struct {
	Publish        int        `json:"publish"`
	PublishDetails rateDetail `json:"publish_details"`
	Deliver        int        `json:"deliver"`
	DeliverDetails rateDetail `json:"deliver_details"`
	Ack            int        `json:"ack"`
}

type rateDetail struct {
	Rate float64 `json:"rate"`
}

func (q apiQueue) publishRate() float64 {
	if q.MessageStats == nil {
		return 0
	}
	return q.MessageStats.PublishDetails.Rate
}

func (q apiQueue) deliverRate() float64 {
	if q.MessageStats == nil {
		return 0
	}
	return q.MessageStats.DeliverDetails.Rate
}

func (q apiQueue) totalPublished() int {
	if q.MessageStats == nil {
		return 0
	}
	return q.MessageStats.Publish
}

func fetchQueues(apiURL, user, pass string) (map[string]apiQueue, error) {
	req, err := http.NewRequest("GET", apiURL+"/api/queues/%2F", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(user, pass)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var list []apiQueue
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}

	m := make(map[string]apiQueue, len(list))
	for _, q := range list {
		m[q.Name] = q
	}
	return m, nil
}

// ── Team metadata ─────────────────────────────────────────────────────────────

type teamMeta struct {
	routine string
	work    string
	idle    string
}

var teams = []struct {
	queue string
	meta  teamMeta
}{
	{"team.catering", teamMeta{"Concentrated", "60s", "60s"}},
	{"team.coordinator", teamMeta{"Concentrated", "60s", "60s"}},
	{"team.venue", teamMeta{"Intermittent", "5s", "5s"}},
	{"team.medical", teamMeta{"Intermittent", "5s", "5s"}},
	{"team.security", teamMeta{"Standard", "20s", "5s"}},
	{"team.decoration", teamMeta{"Standard", "20s", "5s"}},
	{"team.photography", teamMeta{"Standard", "20s", "5s"}},
	{"team.music", teamMeta{"Standard", "20s", "5s"}},
	{"team.floral", teamMeta{"Standard", "20s", "5s"}},
	{"team.transport", teamMeta{"Standard", "20s", "5s"}},
}

func routineColor(r string) string {
	switch r {
	case "Concentrated":
		return red
	case "Intermittent":
		return green
	default:
		return yellow
	}
}

// ── Rendering helpers ─────────────────────────────────────────────────────────

// bar renders a fixed-width progress bar scaled to maxVal.
func bar(val, maxVal, width int) string {
	if maxVal <= 0 {
		maxVal = 1
	}
	filled := val * width / maxVal
	if filled > width {
		filled = width
	}
	color := green
	ratio := float64(val) / float64(maxVal)
	switch {
	case ratio > 0.66:
		color = red
	case ratio > 0.33:
		color = yellow
	}
	return color + strings.Repeat("█", filled) + dim + strings.Repeat("░", width-filled) + reset
}

// rateArrow returns a coloured activity indicator for a message rate.
func rateArrow(rate float64) string {
	switch {
	case rate > 10:
		return clr(green, ">>>")
	case rate > 2:
		return clr(yellow, " >>")
	case rate > 0.1:
		return clr(cyan, "  >")
	default:
		return clr(dim, "  ·")
	}
}

func divider(width int) string {
	return clr(dim, strings.Repeat("─", width))
}

// ── Main render ───────────────────────────────────────────────────────────────

func render(qs map[string]apiQueue, start time.Time, speed float64, workers int, prevResults int) int {
	elapsed := time.Since(start)
	simElapsed := time.Duration(float64(elapsed) * speed)

	// Compute max team queue depth for bar scaling.
	maxDepth := 10
	for _, t := range teams {
		if q, ok := qs[t.queue]; ok && q.Messages > maxDepth {
			maxDepth = q.Messages
		}
	}

	validated := qs["events.validated"]
	results := qs["events.results"]

	// Delta since last render — gives a sense of throughput without consuming messages.
	resultsDelta := results.totalPublished() - prevResults

	fmt.Print(clearScreen + cursorHome)

	w := 74 // display width

	// ── Header ────────────────────────────────────────────────────────────────
	fmt.Println(clr(cyan, bold+strings.Repeat("═", w)+reset))
	fmt.Printf("  %s  Live Pipeline Monitor"+
		"   speed %s   elapsed %s   sim time %s\n",
		b("MARRY-ME"),
		clr(yellow, fmt.Sprintf("%.1fx", speed)),
		clr(white, elapsed.Round(time.Second).String()),
		clr(cyan, simElapsed.Round(time.Second).String()),
	)
	fmt.Println(clr(cyan, bold+strings.Repeat("═", w)+reset))

	// ── Stage 1: Coordinator ─────────────────────────────────────────────────
	fmt.Println()
	fmt.Printf("%s  %s\n",
		b(clr(cyan, "[1] COORDINATOR")),
		d("internal/coordinator/coordinator.go"))
	fmt.Println(divider(w))
	fmt.Printf("  Queue %-20s  depth %s%4d%s   pub %s  %5.1f/s\n",
		clr(yellow, "events.validated"),
		yellow, validated.Messages, reset,
		rateArrow(validated.publishRate()), validated.publishRate(),
	)
	fmt.Printf("  %s  reads dataset JSON → sorts events by %s field\n",
		b("Ingest()"), clr(green, "timestamp"))
	fmt.Printf("  Schedules each event with %s, compressed by %s flag\n",
		clr(green, "time.After(delay/speed)"), clr(yellow, "--speed"))
	fmt.Printf("  Runs as a %s so the simulation timer fires concurrently\n",
		clr(green, "goroutine"))

	// ── Stage 2: Organizer ────────────────────────────────────────────────────
	fmt.Println()
	fmt.Printf("%s  %s\n",
		b(clr(cyan, "[2] ORGANIZER")),
		d("internal/organizer/organizer.go"))
	fmt.Println(divider(w))
	fmt.Printf("  Exchange  %s  (direct routing, 10 binding keys)\n",
		clr(yellow, "events.organized"))
	fmt.Printf("  %s  unmarshals event → calls %s\n",
		b("handleDelivery()"), b("drainLocked()"))
	fmt.Printf("  %s  uses %s (O(log n)) — High before Medium before Low\n",
		b("drainLocked()"), clr(green, "container/heap"))
	fmt.Printf("  %s  publishes to exchange with routing key = team name\n",
		b("routeEvent()"))

	// ── Stage 3: Team queues / Worker pools ───────────────────────────────────
	fmt.Println()
	fmt.Printf("%s  %s  /  %s\n",
		b(clr(cyan, "[3] TEAM QUEUES & WORKER POOLS")),
		d("internal/team/manager.go"),
		d("internal/worker/worker.go"))
	fmt.Println(divider(w))
	fmt.Printf("  %s spawns %s per team, each calling %s\n",
		b("setupTeam()"),
		clr(green, fmt.Sprintf("%d worker goroutines", workers)),
		b("worker.Run()"))
	fmt.Printf("  Work/idle state machine: %s sends on %s channel\n",
		d("goroutine"), clr(green, "idleCh"))
	fmt.Printf("  During work phase: event is %s or %s if already expired\n\n",
		clr(yellow, "republished"), clr(red, "reported expired"))

	for _, t := range teams {
		q := qs[t.queue]
		m := t.meta
		rColor := routineColor(m.routine)
		barStr := bar(q.Messages, maxDepth, 14)

		fmt.Printf("  %-20s [%s] %s%3d%s  %s%-12s%s %s(%s/%s)%s  %s %.1f/s%s\n",
			clr(dim, t.queue),
			barStr,
			yellow, q.Messages, reset,
			rColor, m.routine, reset,
			dim, m.work, m.idle, reset,
			cyan, q.deliverRate(), reset,
		)
	}

	// ── Stage 4: Results / Stress Tracker ────────────────────────────────────
	fmt.Println()
	fmt.Printf("%s  %s\n",
		b(clr(cyan, "[4] RESULTS & STRESS TRACKER")),
		d("internal/stress/tracker.go"))
	fmt.Println(divider(w))
	fmt.Printf("  Queue %-20s  depth %s%4d%s   del %s  %5.1f/s\n",
		clr(yellow, "events.results"),
		yellow, results.Messages, reset,
		rateArrow(results.deliverRate()), results.deliverRate(),
	)
	fmt.Printf("  %s  goroutine reads %s wrappers from the queue\n",
		b("Consume()"), clr(green, "ResultEvent"))
	fmt.Printf("  %s  increments completed/expired counters (mutex-guarded)\n",
		b("Record()"))
	fmt.Printf("  %s  stress = expired/total, breakdown by priority & team\n",
		b("Report()"))

	total := results.totalPublished()
	if total > 0 {
		fmt.Printf("\n  Results received: %s%d%s", clr(green, fmt.Sprintf("%d", total)), 0, reset)
		if resultsDelta > 0 {
			fmt.Printf("  (+%s%d%s this tick)", clr(green, fmt.Sprintf("%d", resultsDelta)), 0, reset)
		}
		fmt.Println()
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println(divider(w))
	fmt.Printf("  %s to exit   polling every 500ms via %s\n",
		b("Ctrl+C"),
		clr(cyan, "GET /api/queues  (Management API — no message consumption)"))

	return total
}

// ── Entry point ───────────────────────────────────────────────────────────────

func main() {
	apiURL := flag.String("api-url", "http://localhost:15672", "RabbitMQ Management API base URL")
	user := flag.String("user", "admin", "Management API username")
	pass := flag.String("pass", "password", "Management API password")
	speed := flag.Float64("speed", 1.0, "Speed multiplier used in the marry-me run (display only)")
	workers := flag.Int("workers", 3, "Workers per team used in the marry-me run (display only)")
	refresh := flag.Duration("refresh", 500*time.Millisecond, "Refresh interval")
	flag.Parse()

	// Verify connectivity before entering the render loop.
	if _, err := fetchQueues(*apiURL, *user, *pass); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot reach RabbitMQ Management API at %s\n", *apiURL)
		fmt.Fprintf(os.Stderr, "  - Is RabbitMQ running?  docker-compose up -d rabbitmq\n")
		fmt.Fprintf(os.Stderr, "  - Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(hideCursor)
	defer fmt.Print(showCursor)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	start := time.Now()
	ticker := time.NewTicker(*refresh)
	defer ticker.Stop()

	prevResults := 0

	for {
		select {
		case <-sigCh:
			fmt.Print(clearScreen + cursorHome + showCursor)
			fmt.Println("Monitor stopped.")
			return
		case <-ticker.C:
			qs, err := fetchQueues(*apiURL, *user, *pass)
			if err != nil {
				fmt.Printf("%sAPI error: %v%s\n", red, err, reset)
				continue
			}
			prevResults = render(qs, start, *speed, *workers, prevResults)
		}
	}
}
