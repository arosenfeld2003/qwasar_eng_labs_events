#!/bin/bash

# Run all datasets and collect stress reports

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BIN="$PROJECT_DIR/bin/marry-me"
DATASETS_DIR="$PROJECT_DIR/datasets"
REPORTS_DIR="$PROJECT_DIR/reports"

# Create reports directory
mkdir -p "$REPORTS_DIR"

# Build if needed
if [ ! -f "$BIN" ]; then
    echo "Building application..."
    cd "$PROJECT_DIR"
    go build -o bin/marry-me ./cmd/marry-me
fi

echo "========================================"
echo "  Running All Datasets"
echo "========================================"
echo ""

# Summary array
declare -a RESULTS

for i in 1 2 3 4 5; do
    DATASET="$DATASETS_DIR/dataset_$i.json"

    if [ ! -f "$DATASET" ]; then
        echo "Warning: $DATASET not found, skipping..."
        continue
    fi

    echo ""
    echo "========================================"
    echo "  Dataset $i"
    echo "========================================"

    # Run the simulation
    "$BIN" --dataset="$DATASET" --report="$REPORTS_DIR/report_$i.json"

    # Extract stress level from report if it exists
    if [ -f "$REPORTS_DIR/report_$i.json" ]; then
        STRESS=$(cat "$REPORTS_DIR/report_$i.json" | grep -o '"stress_level":[0-9.]*' | cut -d: -f2)
        RESULTS+=("Dataset $i: ${STRESS}% stress")
    fi
done

echo ""
echo "========================================"
echo "  Summary"
echo "========================================"
for result in "${RESULTS[@]}"; do
    echo "  $result"
done
echo ""
echo "Reports saved to: $REPORTS_DIR/"
echo "========================================"
