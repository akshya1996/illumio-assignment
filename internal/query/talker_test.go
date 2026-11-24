// Tests for TopTalkers and its helper aggregateTalkerBytes. Each test
// builds FlowRecords by hand with only SourceIP, Bytes and StartTime set,
// since those are the only fields these functions use. Most tests use the
// time window [100, 200).

package query

import (
	"net/netip"
	"testing"

	"github.com/akshya1996/illumio-assignment/internal/model"
)

// TestAggregateTalkerBytes checks that bytes are summed per source IP.
// ip1 has two records (500 + 300 = 800) and ip2 has one (200).
func TestAggregateTalkerBytes(t *testing.T) {
	ip1 := netip.MustParseAddr("10.0.0.1")
	ip2 := netip.MustParseAddr("10.0.0.2")

	records := []model.FlowRecord{
		{SourceIP: &ip1, Bytes: 500, StartTime: 100},
		{SourceIP: &ip1, Bytes: 300, StartTime: 150},
		{SourceIP: &ip2, Bytes: 200, StartTime: 120},
	}

	result := aggregateTalkerBytes(records, 100, 200)

	if result[ip1] != 800 {
		t.Errorf("expected 800 bytes for %s, got %d", ip1, result[ip1])
	}

	if result[ip2] != 200 {
		t.Errorf("expected 200 bytes for %s, got %d", ip2, result[ip2])
	}
}

// TestAggregateTalkerBytesTimeWindow checks the edges of the time window
// [100, 200). A record starting exactly at 100 is counted, one starting
// exactly at 200 is not. Expected total: 200 + 300 + 400 = 900.
func TestAggregateTalkerBytesTimeWindow(t *testing.T) {
	ip := netip.MustParseAddr("10.0.0.1")

	records := []model.FlowRecord{
		{SourceIP: &ip, Bytes: 100, StartTime: 99},  // Before window
		{SourceIP: &ip, Bytes: 200, StartTime: 100}, // At start: included
		{SourceIP: &ip, Bytes: 300, StartTime: 150}, // Inside: included
		{SourceIP: &ip, Bytes: 400, StartTime: 199}, // Before end: included
		{SourceIP: &ip, Bytes: 500, StartTime: 200}, // At end: excluded
	}

	result := aggregateTalkerBytes(records, 100, 200)

	if result[ip] != 900 {
		t.Errorf("expected 900 bytes, got %d", result[ip])
	}
}

// TestAggregateTalkerBytesMissingSourceIP checks that a record with no
// source IP is skipped, rather than being counted under the zero address.
func TestAggregateTalkerBytesMissingSourceIP(t *testing.T) {
	ip := netip.MustParseAddr("10.0.0.1")

	records := []model.FlowRecord{
		{SourceIP: &ip, Bytes: 500, StartTime: 100},
		{SourceIP: nil, Bytes: 1000, StartTime: 100}, // No source IP: skipped
	}

	result := aggregateTalkerBytes(records, 100, 200)

	if len(result) != 1 {
		t.Fatalf("expected 1 talker, got %d", len(result))
	}

	if result[ip] != 500 {
		t.Errorf("expected 500 bytes, got %d", result[ip])
	}
}

// TestTopTalkers checks that talkers are ranked by total bytes and cut to
// the top n. Totals are ip2 = 900, ip1 = 500 + 200 = 700 and ip3 = 300, so
// the top 2 should be ip2 then ip1. This also checks that ip1's two records
// are added together before ranking.
func TestTopTalkers(t *testing.T) {
	ip1 := netip.MustParseAddr("10.0.0.1")
	ip2 := netip.MustParseAddr("10.0.0.2")
	ip3 := netip.MustParseAddr("10.0.0.3")

	records := []model.FlowRecord{
		{SourceIP: &ip1, Bytes: 500, StartTime: 100},
		{SourceIP: &ip2, Bytes: 900, StartTime: 100},
		{SourceIP: &ip3, Bytes: 300, StartTime: 100},
		{SourceIP: &ip1, Bytes: 200, StartTime: 110},
	}

	result := TopTalkers(records, 100, 200, 2)

	if len(result) != 2 {
		t.Fatalf("expected 2 talkers, got %d", len(result))
	}

	if result[0].SourceIP != ip2 || result[0].Bytes != 900 {
		t.Errorf("expected first talker %s with 900 bytes, got %+v", ip2, result[0])
	}

	if result[1].SourceIP != ip1 || result[1].Bytes != 700 {
		t.Errorf("expected second talker %s with 700 bytes, got %+v", ip1, result[1])
	}
}

// TestTopTalkersNExceedsAvailable checks that asking for more talkers than
// exist (10 here, with only 2 IPs) returns all of them without an error or
// panic.
func TestTopTalkersNExceedsAvailable(t *testing.T) {
	ip1 := netip.MustParseAddr("10.0.0.1")
	ip2 := netip.MustParseAddr("10.0.0.2")

	records := []model.FlowRecord{
		{SourceIP: &ip1, Bytes: 500, StartTime: 100},
		{SourceIP: &ip2, Bytes: 300, StartTime: 100},
	}

	result := TopTalkers(records, 100, 200, 10)

	if len(result) != 2 {
		t.Fatalf("expected 2 talkers, got %d", len(result))
	}
}

// TestTopTalkersNonPositiveN checks that n = 0 and a negative n both return
// no talkers.
func TestTopTalkersNonPositiveN(t *testing.T) {
	ip := netip.MustParseAddr("10.0.0.1")

	records := []model.FlowRecord{
		{SourceIP: &ip, Bytes: 500, StartTime: 100},
	}

	for _, n := range []int{0, -1} {
		result := TopTalkers(records, 100, 200, n)

		if len(result) != 0 {
			t.Errorf("expected 0 talkers for n=%d, got %d", n, len(result))
		}
	}
}

// TestTopTalkersEqualBytes checks the tie-break rule. All three IPs sent the
// same number of bytes, and the records are deliberately out of order, so
// the result must be sorted by IP (ip1, ip2, ip3) to be the same every
// run.
func TestTopTalkersEqualBytes(t *testing.T) {
	ip1 := netip.MustParseAddr("10.0.0.1")
	ip2 := netip.MustParseAddr("10.0.0.2")
	ip3 := netip.MustParseAddr("10.0.0.3")

	records := []model.FlowRecord{
		{SourceIP: &ip3, Bytes: 500, StartTime: 100},
		{SourceIP: &ip1, Bytes: 500, StartTime: 100},
		{SourceIP: &ip2, Bytes: 500, StartTime: 100},
	}

	result := TopTalkers(records, 100, 200, 3)

	if len(result) != 3 {
		t.Fatalf("expected 3 talkers, got %d", len(result))
	}

	expected := []netip.Addr{ip1, ip2, ip3}

	for i, want := range expected {
		if result[i].SourceIP != want {
			t.Errorf("result[%d]: expected %s, got %s", i, want, result[i].SourceIP)
		}
	}
}
