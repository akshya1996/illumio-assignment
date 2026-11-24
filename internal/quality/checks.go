package quality

import (
	"fmt"
	"sort"

	"github.com/akshya1996/illumio-assignment/internal/model"
)

// FindImpossibleValues returns the indexes of records with negative packet or byte
// counts, or with a start time after the end time.
func FindImpossibleValues(records []model.FlowRecord) []int {
	var invalid []int

	for i, record := range records {
		if record.Packets < 0 ||
			record.Bytes < 0 ||
			record.StartTime > record.EndTime {
			invalid = append(invalid, i)
		}
	}

	return invalid
}

// FindDuplicates returns the indexes of repeated records while preserving the
// first occurrence of each exact record.
func FindDuplicates(records []model.FlowRecord) []int {
	seen := make(map[string]bool)
	var duplicates []int

	for i, record := range records {
		sourceIP, destinationIP := "", ""

		if record.SourceIP != nil {
			sourceIP = record.SourceIP.String()
		}
		if record.DestinationIP != nil {
			destinationIP = record.DestinationIP.String()
		}

		sourcePort, destinationPort := -1, -1

		if record.SourcePort != nil {
			sourcePort = *record.SourcePort
		}
		if record.DestinationPort != nil {
			destinationPort = *record.DestinationPort
		}

		key := fmt.Sprintf(
			"%s|%s|%d|%d|%d|%d|%d|%d|%d|%s",
			sourceIP,
			destinationIP,
			sourcePort,
			destinationPort,
			record.Protocol,
			record.Packets,
			record.Bytes,
			record.StartTime,
			record.EndTime,
			record.Action,
		)

		if seen[key] {
			duplicates = append(duplicates, i)
		} else {
			seen[key] = true
		}
	}

	return duplicates
}

// FindOneSidedTCP returns TCP flow indexes that do not have a matching reverse
// flow with swapped source and destination IP addresses and ports.
func FindOneSidedTCP(records []model.FlowRecord) []int {
	type flowKey struct {
		srcIP   string
		dstIP   string
		srcPort int
		dstPort int
	}

	seen := make(map[flowKey]bool)

	for _, record := range records {
		if record.Protocol != 6 ||
			record.SourceIP == nil ||
			record.DestinationIP == nil ||
			record.SourcePort == nil ||
			record.DestinationPort == nil {
			continue
		}

		key := flowKey{
			srcIP:   record.SourceIP.String(),
			dstIP:   record.DestinationIP.String(),
			srcPort: *record.SourcePort,
			dstPort: *record.DestinationPort,
		}

		seen[key] = true
	}

	var oneSided []int

	for i, record := range records {
		if record.Protocol != 6 ||
			record.SourceIP == nil ||
			record.DestinationIP == nil ||
			record.SourcePort == nil ||
			record.DestinationPort == nil {
			continue
		}

		reverse := flowKey{
			srcIP:   record.DestinationIP.String(),
			dstIP:   record.SourceIP.String(),
			srcPort: *record.DestinationPort,
			dstPort: *record.SourcePort,
		}

		if !seen[reverse] {
			oneSided = append(oneSided, i)
		}
	}

	return oneSided
}

// FindTrafficSpikes groups records into 60-second buckets per source IP and
// returns records in buckets whose byte volume exceeds three times the median
// bucket volume for that source IP.
func FindTrafficSpikes(records []model.FlowRecord) []int {
	const bucketSize int64 = 60
	const spikeMultiplier int64 = 3

	type bucketKey struct {
		sourceIP string
		bucket   int64
	}

	bucketBytes := make(map[bucketKey]int64)
	bucketRecords := make(map[bucketKey][]int)

	for i, record := range records {
		if record.SourceIP == nil {
			continue
		}

		key := bucketKey{
			sourceIP: record.SourceIP.String(),
			bucket:   record.StartTime / bucketSize,
		}

		bucketBytes[key] += record.Bytes
		bucketRecords[key] = append(bucketRecords[key], i)
	}

	// Collect bucket totals for each source IP.
	sourceBuckets := make(map[string][]int64)

	for key, total := range bucketBytes {
		sourceBuckets[key.sourceIP] = append(
			sourceBuckets[key.sourceIP], total,
		)
	}

	// Calculate the median and flag unusually large buckets.
	var spikes []int

	for sourceIP, totals := range sourceBuckets {
		sort.Slice(totals, func(i, j int) bool {
			return totals[i] < totals[j]
		})

		middle := len(totals) / 2
		median := totals[middle]

		if len(totals)%2 == 0 {
			median = (totals[middle-1] + totals[middle]) / 2
		}

		for key, total := range bucketBytes {
			if key.sourceIP == sourceIP &&
				total > spikeMultiplier*median {
				spikes = append(spikes, bucketRecords[key]...)
			}
		}
	}

	sort.Ints(spikes)
	return spikes
}
