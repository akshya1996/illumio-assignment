package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"github.com/akshya1996/illumio-assignment/internal/model"
	"github.com/akshya1996/illumio-assignment/internal/parser"
	"github.com/akshya1996/illumio-assignment/internal/quality"
)

// loadRecords opens and parses a flow log, preserving parser errors separately
// from file-level errors.
func loadRecords(path string) ([]model.FlowRecord, []error, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	records, parseErrors := parser.Parse(file)
	return records, parseErrors, nil
}

// loadGroundTruth reads the ground-truth CSV and groups record line numbers by
// anomaly category.
func loadGroundTruth(path string) (map[string][]int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	groundTruth := make(map[string][]int)

	for _, row := range rows[1:] {
		line, err := strconv.Atoi(row[0])
		if err != nil {
			return nil, err
		}

		category := row[1]
		groundTruth[category] = append(groundTruth[category], line)
	}

	return groundTruth, nil
}

// expectedIndexes converts flow-log line numbers to record indexes so they can
// be compared directly with indexes returned by the quality checks.
func expectedIndexes(groundTruth map[string][]int, category string) map[int]bool {
	expected := make(map[int]bool)

	for _, line := range groundTruth[category] {
		// The generated log has a header on line 1, so line N is record N-2.
		expected[line-2] = true
	}

	return expected
}

// compareResults calculates true positives, false positives, and false negatives
// for a detected record-index set.
func compareResults(detected []int, expected map[int]bool) (int, int, int) {
	truePositives := 0
	falsePositives := 0

	for _, index := range detected {
		if expected[index] {
			truePositives++
		} else {
			falsePositives++
		}
	}

	falseNegatives := len(expected) - truePositives

	return truePositives, falsePositives, falseNegatives
}

// printMetrics prints a compact evaluation summary for one anomaly category.
func printMetrics(category string, detected []int, expected map[int]bool) {
	truePositives, falsePositives, falseNegatives :=
		compareResults(detected, expected)

	catchRate := 0.0
	if len(expected) > 0 {
		catchRate = float64(truePositives) / float64(len(expected)) * 100
	}

	fmt.Printf(
		"%-20s Expected: %d | Detected: %d | False positives: %d | Missed: %d | Catch rate: %.1f%%\n",
		category,
		len(expected),
		truePositives,
		falsePositives,
		falseNegatives,
		catchRate,
	)
}

func main() {
	records, parseErrors, err := loadRecords("samples/quality_flow.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load records: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Valid records: %d\n", len(records))
	fmt.Printf("Parser errors: %d\n", len(parseErrors))

	for _, parseErr := range parseErrors {
		fmt.Printf("  - %v\n", parseErr)
	}

	impossible := quality.FindImpossibleValues(records)
	duplicates := quality.FindDuplicates(records)
	oneSidedTCP := quality.FindOneSidedTCP(records)
	trafficSpikes := quality.FindTrafficSpikes(records)

	fmt.Printf("\nImpossible values: %v\n", impossible)
	fmt.Printf("Duplicates: %v\n", duplicates)
	fmt.Printf("One-sided TCP: %v\n", oneSidedTCP)
	fmt.Printf("Traffic spikes: %v\n", trafficSpikes)

	groundTruth, err := loadGroundTruth("samples/quality_ground_truth.csv")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load ground truth: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n--- Data Quality Evaluation ---")

	printMetrics(
		"impossible_values",
		impossible,
		expectedIndexes(groundTruth, "impossible_values"),
	)

	printMetrics(
		"duplicate",
		duplicates,
		expectedIndexes(groundTruth, "duplicate"),
	)

	printMetrics(
		"one_sided_tcp",
		oneSidedTCP,
		expectedIndexes(groundTruth, "one_sided_tcp"),
	)

	printMetrics(
		"traffic_spike",
		trafficSpikes,
		expectedIndexes(groundTruth, "traffic_spike"),
	)

	brokenExpected := len(groundTruth["broken_fields"])
	brokenDetected := len(parseErrors)

	// Clamp broken-field extras and misses to zero for a usable catch-rate report.
	brokenFalsePositives := brokenDetected - brokenExpected
	if brokenFalsePositives < 0 {
		brokenFalsePositives = 0
	}

	brokenMissed := brokenExpected - brokenDetected
	if brokenMissed < 0 {
		brokenMissed = 0
	}

	brokenCatchRate := 0.0
	if brokenExpected > 0 {
		brokenCatchRate = float64(brokenDetected-brokenFalsePositives) /
			float64(brokenExpected) * 100
	}

	fmt.Printf(
		"%-20s Expected: %d | Detected: %d | False positives: %d | Missed: %d | Catch rate: %.1f%%\n",
		"broken_fields",
		brokenExpected,
		brokenDetected,
		brokenFalsePositives,
		brokenMissed,
		brokenCatchRate,
	)

	totalExpected := 0
	totalDetected := 0

	for _, lines := range groundTruth {
		totalExpected += len(lines)
	}

	totalDetected += len(impossible)
	totalDetected += len(duplicates)
	totalDetected += len(oneSidedTCP)
	totalDetected += len(trafficSpikes)
	totalDetected += brokenDetected - brokenFalsePositives

	overallCatchRate := 0.0
	if totalExpected > 0 {
		overallCatchRate = float64(totalDetected) /
			float64(totalExpected) * 100
	}

	fmt.Printf("\nOverall catch rate: %.1f%% (%d/%d)\n",
		overallCatchRate, totalDetected, totalExpected)

}
