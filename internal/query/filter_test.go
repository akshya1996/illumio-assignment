// Tests for FilterRecords. Each test builds a small slice of FlowRecords by
// hand, setting only the fields the test cares about, then checks how many
// records the filter keeps and that they are the right ones.

package query

import (
	"net/netip"
	"testing"

	"github.com/akshya1996/illumio-assignment/internal/model"
)

// TestFilterBySourceIP checks that a source IP filter keeps only the records
// sent from that address (2 of the 3 here).
func TestFilterBySourceIP(t *testing.T) {
	ip1 := netip.MustParseAddr("10.0.0.1")
	ip2 := netip.MustParseAddr("10.0.0.2")

	records := []model.FlowRecord{
		{SourceIP: &ip1},
		{SourceIP: &ip2},
		{SourceIP: &ip1},
	}

	filter := Filter{
		SourceIP: &ip1,
	}

	result := FilterRecords(records, filter)

	if len(result) != 2 {
		t.Fatalf("expected 2 records, got %d", len(result))
	}

	for _, record := range result {
		if record.SourceIP == nil || *record.SourceIP != ip1 {
			t.Errorf("expected source IP %s, got %v", ip1, record.SourceIP)
		}
	}
}

// TestFilterByDestinationIP checks that a destination IP filter keeps only
// the records sent to that address (2 of the 3 here).
func TestFilterByDestinationIP(t *testing.T) {
	ip1 := netip.MustParseAddr("10.0.0.1")
	ip2 := netip.MustParseAddr("10.0.0.2")

	records := []model.FlowRecord{
		{DestinationIP: &ip1},
		{DestinationIP: &ip2},
		{DestinationIP: &ip1},
	}

	filter := Filter{
		DestinationIP: &ip1,
	}

	result := FilterRecords(records, filter)

	if len(result) != 2 {
		t.Fatalf("expected 2 records, got %d", len(result))
	}

	for _, record := range result {
		if record.DestinationIP == nil || *record.DestinationIP != ip1 {
			t.Errorf("expected destination IP %s, got %v", ip1, record.DestinationIP)
		}
	}
}

// TestFilterBySourcePort checks that a source port filter keeps only the
// records with that port. The record with no source port (nil) must be left
// out, not treated as a match.
func TestFilterBySourcePort(t *testing.T) {
	port1 := 443
	port2 := 8080

	records := []model.FlowRecord{
		{SourcePort: &port1},
		{SourcePort: &port2},
		{SourcePort: nil},
		{SourcePort: &port1},
	}

	filter := Filter{
		SourcePort: &port1,
	}

	result := FilterRecords(records, filter)

	if len(result) != 2 {
		t.Fatalf("expected 2 records, got %d", len(result))
	}

	for _, record := range result {
		if record.SourcePort == nil {
			t.Fatal("expected source port, got nil")
		}

		if *record.SourcePort != port1 {
			t.Errorf("expected source port %d, got %d", port1, *record.SourcePort)
		}
	}
}

// TestFilterByDestinationPort checks that a destination port filter keeps
// only the records with that port. The record with no destination port
// (nil) must be left out, not treated as a match.
func TestFilterByDestinationPort(t *testing.T) {
	port1 := 443
	port2 := 8080

	records := []model.FlowRecord{
		{DestinationPort: &port1},
		{DestinationPort: &port2},
		{DestinationPort: nil},
		{DestinationPort: &port1},
	}

	filter := Filter{
		DestinationPort: &port1,
	}

	result := FilterRecords(records, filter)

	if len(result) != 2 {
		t.Fatalf("expected 2 records, got %d", len(result))
	}

	for _, record := range result {
		if record.DestinationPort == nil {
			t.Fatal("expected destination port, got nil")
		}

		if *record.DestinationPort != port1 {
			t.Errorf("expected destination port %d, got %d", port1, *record.DestinationPort)
		}
	}
}

// TestFilterByProtocol checks that a protocol filter (TCP = 6 here) keeps
// only records with that protocol number and drops the UDP (17) record.
func TestFilterByProtocol(t *testing.T) {
	tcp := 6
	udp := 17

	records := []model.FlowRecord{
		{Protocol: tcp},
		{Protocol: udp},
		{Protocol: tcp},
	}

	filter := Filter{
		Protocol: &tcp,
	}

	result := FilterRecords(records, filter)

	if len(result) != 2 {
		t.Fatalf("expected 2 records, got %d", len(result))
	}

	for _, record := range result {
		if record.Protocol != tcp {
			t.Errorf("expected protocol %d, got %d", tcp, record.Protocol)
		}
	}
}

// TestFilterByAction checks that an action filter keeps only the ACCEPT
// records and drops the REJECT one.
func TestFilterByAction(t *testing.T) {
	accept := "ACCEPT"
	reject := "REJECT"

	records := []model.FlowRecord{
		{Action: accept},
		{Action: reject},
		{Action: accept},
	}

	filter := Filter{
		Action: &accept,
	}

	result := FilterRecords(records, filter)

	if len(result) != 2 {
		t.Fatalf("expected 2 records, got %d", len(result))
	}

	for _, record := range result {
		if record.Action != accept {
			t.Errorf("expected action %s, got %s", accept, record.Action)
		}
	}
}

// TestFilterByMultipleCriteria checks that when several filter fields are
// set, a record must match all of them. The first record matches every
// field; each of the others differs in exactly one field, so only the first
// should be returned.
func TestFilterByMultipleCriteria(t *testing.T) {
	srcIP := netip.MustParseAddr("10.0.0.1")
	otherIP := netip.MustParseAddr("10.0.0.2")

	port := 443
	protocol := 6
	action := "ACCEPT"

	records := []model.FlowRecord{
		// Matches all criteria
		{
			SourceIP:        &srcIP,
			DestinationPort: &port,
			Protocol:        protocol,
			Action:          action,
		},
		// Wrong source IP
		{
			SourceIP:        &otherIP,
			DestinationPort: &port,
			Protocol:        protocol,
			Action:          action,
		},
		// Missing destination port (nil), so it can't match port 443
		{
			SourceIP:        &srcIP,
			DestinationPort: nil,
			Protocol:        protocol,
			Action:          action,
		},
		// Wrong protocol
		{
			SourceIP:        &srcIP,
			DestinationPort: &port,
			Protocol:        17,
			Action:          action,
		},
		// Wrong action
		{
			SourceIP:        &srcIP,
			DestinationPort: &port,
			Protocol:        protocol,
			Action:          "REJECT",
		},
	}

	filter := Filter{
		SourceIP:        &srcIP,
		DestinationPort: &port,
		Protocol:        &protocol,
		Action:          &action,
	}

	result := FilterRecords(records, filter)

	if len(result) != 1 {
		t.Fatalf("expected 1 matching record, got %d", len(result))
	}

	if result[0].SourceIP == nil || *result[0].SourceIP != srcIP {
		t.Errorf("unexpected source IP in result")
	}
}

// TestFilterWithNoCriteria checks that an empty Filter (no fields set)
// returns every record.
func TestFilterWithNoCriteria(t *testing.T) {
	records := []model.FlowRecord{
		{Protocol: 6, Action: "ACCEPT"},
		{Protocol: 17, Action: "REJECT"},
	}

	result := FilterRecords(records, Filter{})

	if len(result) != len(records) {
		t.Fatalf("expected %d records, got %d", len(records), len(result))
	}
}

// TestFilterWithNoMatches checks that a filter no record matches returns an
// empty result rather than failing.
func TestFilterWithNoMatches(t *testing.T) {
	ip1 := netip.MustParseAddr("10.0.0.1")
	ip2 := netip.MustParseAddr("10.0.0.2")

	records := []model.FlowRecord{
		{SourceIP: &ip1},
		{SourceIP: &ip1},
	}

	filter := Filter{
		SourceIP: &ip2,
	}

	result := FilterRecords(records, filter)

	if len(result) != 0 {
		t.Errorf("expected 0 records, got %d", len(result))
	}
}
