package main

import (
	"fmt"
	"math/rand/v2"
	"net/netip"
	"os"

	"github.com/akshya1996/illumio-assignment/internal/model"
)

const seed int64 = 42

// generateCleanRecords creates synthetic flow records and adds reverse TCP flows
// so normal TCP traffic is represented in both directions.
func generateCleanRecords(rng *rand.Rand, count int) []model.FlowRecord {
	sourceIPs := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	destinationIPs := []string{"10.0.1.1", "10.0.1.2", "10.0.1.3"}

	records := make([]model.FlowRecord, 0, count)

	for i := 0; i < count; i++ {
		srcIP := netip.MustParseAddr(sourceIPs[rng.IntN(len(sourceIPs))])
		dstIP := netip.MustParseAddr(destinationIPs[rng.IntN(len(destinationIPs))])

		srcPort := 50000 + rng.IntN(1000)
		dstPort := []int{80, 443, 8080}[rng.IntN(3)]

		packets := int64(10)
		bytes := int64(1000)
		start := int64(1000 + int64(i)*60)
		end := start + int64(1+rng.IntN(300))

		record := model.FlowRecord{
			SourceIP:        &srcIP,
			DestinationIP:   &dstIP,
			SourcePort:      &srcPort,
			DestinationPort: &dstPort,
			Protocol:        []int{6, 17}[rng.IntN(2)],
			Packets:         packets,
			Bytes:           bytes,
			StartTime:       start,
			EndTime:         end,
			Action:          []string{"ACCEPT", "REJECT"}[rng.IntN(2)],
		}

		records = append(records, record)

		// Add the corresponding reverse flow for TCP traffic.
		if record.Protocol == 6 {
			reverse := record

			reverse.SourceIP = record.DestinationIP
			reverse.DestinationIP = record.SourceIP
			reverse.SourcePort = record.DestinationPort
			reverse.DestinationPort = record.SourcePort

			records = append(records, reverse)
		}

	}

	return records
}

// injectImpossibleValues appends records with negative counts or reversed times.
func injectImpossibleValues(records []model.FlowRecord) []model.FlowRecord {
	if len(records) == 0 {
		return records
	}

	// Negative packets
	negativePackets := records[0]
	negativePackets.Packets = -5

	// Negative bytes
	negativeBytes := records[0]
	negativeBytes.Bytes = -500

	// Start time greater than end time
	invalidTime := records[0]
	invalidTime.StartTime = 200
	invalidTime.EndTime = 150

	records = append(records,
		negativePackets,
		negativeBytes,
		invalidTime,
	)

	return records
}

// injectDuplicates appends an exact copy of the first clean record.
func injectDuplicates(records []model.FlowRecord) []model.FlowRecord {
	if len(records) == 0 {
		return records
	}

	// Append an exact copy of the first record.
	records = append(records, records[0])

	return records
}

// injectOneSidedTCP appends a TCP flow without a matching reverse flow.
func injectOneSidedTCP(records []model.FlowRecord) []model.FlowRecord {
	srcIP := netip.MustParseAddr("10.0.0.50")
	dstIP := netip.MustParseAddr("10.0.1.50")
	srcPort := 55000
	dstPort := 443

	record := model.FlowRecord{
		SourceIP:        &srcIP,
		DestinationIP:   &dstIP,
		SourcePort:      &srcPort,
		DestinationPort: &dstPort,
		Protocol:        6,
		Packets:         10,
		Bytes:           1000,
		StartTime:       2000,
		EndTime:         2100,
		Action:          "ACCEPT",
	}

	return append(records, record)
}

// injectTrafficSpike creates one small baseline for a source IP and one later
// bucket containing substantially more traffic.
func injectTrafficSpike(records []model.FlowRecord) []model.FlowRecord {
	srcIP := netip.MustParseAddr("10.0.0.60")
	dstIP := netip.MustParseAddr("10.0.1.60")

	for i := 0; i < 4; i++ {
		srcPort := 56000
		dstPort := 443

		records = append(records, model.FlowRecord{
			SourceIP:        &srcIP,
			DestinationIP:   &dstIP,
			SourcePort:      &srcPort,
			DestinationPort: &dstPort,
			Protocol:        17,
			Packets:         10,
			Bytes:           100,
			StartTime:       int64(4980 + i*60),
			EndTime:         int64(5000 + i*60),
			Action:          "ACCEPT",
		})
	}

	srcPort := 56000
	dstPort := 443

	records = append(records, model.FlowRecord{
		SourceIP:        &srcIP,
		DestinationIP:   &dstIP,
		SourcePort:      &srcPort,
		DestinationPort: &dstPort,
		Protocol:        17,
		Packets:         100,
		Bytes:           10000,
		StartTime:       5220,
		EndTime:         5270,
		Action:          "ACCEPT",
	})

	return records
}

// writeMalformedRecord appends a parser-error case containing an invalid IP.
func writeMalformedRecord(file *os.File) error {
	_, err := fmt.Fprintln(file,
		"2 123456789 eni-bad not-an-ip 10.0.1.1 50000 443 6 10 500 100 150 ACCEPT OK",
	)
	return err
}

// writeRecords writes the flow-log header and serializes each generated record.
func writeRecords(file *os.File, records []model.FlowRecord) error {
	_, err := fmt.Fprintln(file,
		"version account-id interface-id srcaddr dstaddr srcport dstport protocol packets bytes start end action log-status",
	)
	if err != nil {
		return err
	}

	for i, record := range records {
		_, err := fmt.Fprintf(
			file,
			"2 123456789 eni-%03d %s %s %d %d %d %d %d %d %d %s OK\n",
			i,
			record.SourceIP,
			record.DestinationIP,
			*record.SourcePort,
			*record.DestinationPort,
			record.Protocol,
			record.Packets,
			record.Bytes,
			record.StartTime,
			record.EndTime,
			record.Action,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	rng := rand.New(rand.NewPCG(uint64(seed), uint64(seed)))

	records := generateCleanRecords(rng, 100)
	cleanRecordCount := len(records)

	records = injectImpossibleValues(records)
	records = injectDuplicates(records)
	records = injectOneSidedTCP(records)
	records = injectTrafficSpike(records)

	file, err := os.Create("samples/quality_flow.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	if err := writeRecords(file, records); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write records: %v\n", err)
		os.Exit(1)
	}

	if err := writeMalformedRecord(file); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write malformed record: %v\n", err)
		os.Exit(1)
	}

	groundTruthFile, err := os.Create("samples/quality_ground_truth.csv")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create ground truth file: %v\n", err)
		os.Exit(1)
	}
	defer groundTruthFile.Close()

	if err := writeGroundTruth(groundTruthFile, cleanRecordCount); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write ground truth: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d records in samples/quality_flow.log\n", len(records))
}

// writeGroundTruth writes the line numbers and categories used for evaluation.
func writeGroundTruth(file *os.File, cleanRecordCount int) error {
	_, err := fmt.Fprintln(file, "line,category")
	if err != nil {
		return err
	}

	labels := []struct {
		line     int
		category string
	}{
		{cleanRecordCount + 2, "impossible_values"},
		{cleanRecordCount + 3, "impossible_values"},
		{cleanRecordCount + 4, "impossible_values"},
		{cleanRecordCount + 5, "duplicate"},
		{cleanRecordCount + 6, "one_sided_tcp"},
		{cleanRecordCount + 11, "traffic_spike"},
		{cleanRecordCount + 12, "broken_fields"},
	}

	for _, label := range labels {
		if _, err := fmt.Fprintf(file, "%d,%s\n", label.line, label.category); err != nil {
			return err
		}
	}

	return nil
}
