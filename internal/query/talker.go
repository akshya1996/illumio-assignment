package query

import (
	"net/netip"
	"sort"

	"github.com/akshya1996/illumio-assignment/internal/model"
)

// Talker is one source IP and the total number of bytes it sent during the
// requested time window.
type Talker struct {
	SourceIP netip.Addr
	Bytes    int64
}

// aggregateTalkerBytes adds up the bytes sent by each source IP for records
// that started inside the window [from, to): from is included and to is
// not. Only StartTime is checked, so a flow that starts inside the window
// counts in full even if it ends after the window closes.
//
// Records with no source IP are skipped, since they cannot be credited to
// any talker.
func aggregateTalkerBytes(records []model.FlowRecord, from, to int64) map[netip.Addr]int64 {
	totals := make(map[netip.Addr]int64)

	for _, record := range records {
		if record.SourceIP == nil {
			continue
		}

		// Keep only records with from <= StartTime < to
		if record.StartTime < from || record.StartTime >= to {
			continue
		}

		totals[*record.SourceIP] += record.Bytes
	}

	return totals
}

// TopTalkers returns the n source IPs that sent the most bytes in the time
// window [from, to), with the largest first. See aggregateTalkerBytes for
// which records are counted.
//
// IPs with the same byte total are sorted by address, lowest first, so the
// output is the same on every run (Go map order is random). IPv4 addresses
// sort before IPv6.
//
// If fewer than n IPs sent traffic, all of them are returned. If n is 0 or
// negative, the result is an empty, non-nil slice.
func TopTalkers(records []model.FlowRecord, from, to int64, n int) []Talker {
	if n <= 0 {
		return []Talker{}
	}

	totals := aggregateTalkerBytes(records, from, to)

	// Turn the map into a slice so it can be sorted
	talkers := make([]Talker, 0, len(totals))

	for ip, bytes := range totals {
		talkers = append(talkers, Talker{
			SourceIP: ip,
			Bytes:    bytes,
		})
	}

	// Sort by bytes, largest first. On a tie, sort by IP, lowest first.
	sort.Slice(talkers, func(i, j int) bool {
		if talkers[i].Bytes == talkers[j].Bytes {
			return talkers[i].SourceIP.Less(talkers[j].SourceIP)
		}

		return talkers[i].Bytes > talkers[j].Bytes
	})

	// Keep only the top n
	if n < len(talkers) {
		talkers = talkers[:n]
	}

	return talkers
}
