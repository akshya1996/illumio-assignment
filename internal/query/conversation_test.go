// Tests for CountConversations. Each test builds FlowRecords by hand and
// checks how they are grouped into conversations and counted.

package query

import (
	"net/netip"
	"testing"

	"github.com/akshya1996/illumio-assignment/internal/model"
)

// TestCountConversations checks that identical records are grouped into a
// single conversation. Three records with the same IPs, ports and protocol
// should produce one conversation with a count of 3.
func TestCountConversations(t *testing.T) {
	srcIP := netip.MustParseAddr("10.0.0.1")
	dstIP := netip.MustParseAddr("10.0.0.2")
	srcPort := 5000
	dstPort := 443

	records := []model.FlowRecord{
		{
			SourceIP:        &srcIP,
			DestinationIP:   &dstIP,
			SourcePort:      &srcPort,
			DestinationPort: &dstPort,
			Protocol:        6,
		},
		{
			SourceIP:        &srcIP,
			DestinationIP:   &dstIP,
			SourcePort:      &srcPort,
			DestinationPort: &dstPort,
			Protocol:        6,
		},
		{
			SourceIP:        &srcIP,
			DestinationIP:   &dstIP,
			SourcePort:      &srcPort,
			DestinationPort: &dstPort,
			Protocol:        6,
		},
	}

	result := CountConversations(records)

	// The key we expect all three records to be grouped under
	conversation := Conversation{
		SourceIP:               srcIP,
		SourceIPPresent:        true,
		DestinationIP:          dstIP,
		DestinationIPPresent:   true,
		SourcePort:             srcPort,
		SourcePortPresent:      true,
		DestinationPort:        dstPort,
		DestinationPortPresent: true,
		Protocol:               6,
	}

	if got := result[conversation]; got != 3 {
		t.Errorf("expected conversation count 3, got %d", got)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 unique conversation, got %d", len(result))
	}
}

// TestCountDifferentConversations checks that every field is part of the
// conversation key. Each record after the first differs from it in exactly
// one field, so all 6 records should be counted as separate conversations.
func TestCountDifferentConversations(t *testing.T) {
	srcIP := netip.MustParseAddr("10.0.0.1")
	dstIP := netip.MustParseAddr("10.0.0.2")
	otherIP := netip.MustParseAddr("10.0.0.3")

	srcPort := 5000
	dstPort := 443
	otherPort := 8080

	records := []model.FlowRecord{
		// Original conversation
		{
			SourceIP: &srcIP, DestinationIP: &dstIP,
			SourcePort: &srcPort, DestinationPort: &dstPort,
			Protocol: 6,
		},
		// Different source IP
		{
			SourceIP: &otherIP, DestinationIP: &dstIP,
			SourcePort: &srcPort, DestinationPort: &dstPort,
			Protocol: 6,
		},
		// Different destination IP
		{
			SourceIP: &srcIP, DestinationIP: &otherIP,
			SourcePort: &srcPort, DestinationPort: &dstPort,
			Protocol: 6,
		},
		// Different source port
		{
			SourceIP: &srcIP, DestinationIP: &dstIP,
			SourcePort: &otherPort, DestinationPort: &dstPort,
			Protocol: 6,
		},
		// Different destination port
		{
			SourceIP: &srcIP, DestinationIP: &dstIP,
			SourcePort: &srcPort, DestinationPort: &otherPort,
			Protocol: 6,
		},
		// Different protocol
		{
			SourceIP: &srcIP, DestinationIP: &dstIP,
			SourcePort: &srcPort, DestinationPort: &dstPort,
			Protocol: 17,
		},
	}

	result := CountConversations(records)

	if len(result) != 6 {
		t.Fatalf("expected 6 unique conversations, got %d", len(result))
	}
}

// TestCountReverseDirectionSeparately checks that direction matters. Traffic
// from ip1:port1 to ip2:port2 and the reply from ip2:port2 to ip1:port1 are
// counted as two conversations, each with a count of 1.
func TestCountReverseDirectionSeparately(t *testing.T) {
	ip1 := netip.MustParseAddr("10.0.0.1")
	ip2 := netip.MustParseAddr("10.0.0.2")
	port1 := 5000
	port2 := 443

	records := []model.FlowRecord{
		// Request: ip1:port1 -> ip2:port2
		{
			SourceIP: &ip1, DestinationIP: &ip2,
			SourcePort: &port1, DestinationPort: &port2,
			Protocol: 6,
		},
		// Reply: ip2:port2 -> ip1:port1
		{
			SourceIP: &ip2, DestinationIP: &ip1,
			SourcePort: &port2, DestinationPort: &port1,
			Protocol: 6,
		},
	}

	result := CountConversations(records)

	if len(result) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(result))
	}

	for conversation, count := range result {
		if count != 1 {
			t.Errorf("expected count 1 for conversation %+v, got %d", conversation, count)
		}
	}
}

// TestCountMissingPortVsZero checks that a missing source port (nil) and a
// real source port 0 are counted as different conversations. This is why
// Conversation has the ...Present flags; without them both would have
// SourcePort 0 and be merged.
func TestCountMissingPortVsZero(t *testing.T) {
	srcIP := netip.MustParseAddr("10.0.0.1")
	dstIP := netip.MustParseAddr("10.0.0.2")
	zero := 0
	dstPort := 443

	records := []model.FlowRecord{
		{
			SourceIP:        &srcIP,
			DestinationIP:   &dstIP,
			SourcePort:      nil, // no source port in the log
			DestinationPort: &dstPort,
			Protocol:        6,
		},
		{
			SourceIP:        &srcIP,
			DestinationIP:   &dstIP,
			SourcePort:      &zero, // source port is really 0
			DestinationPort: &dstPort,
			Protocol:        6,
		},
	}

	result := CountConversations(records)

	if len(result) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(result))
	}
}

// TestCountConversationsEmptyInput checks that nil input returns no
// conversations.
func TestCountConversationsEmptyInput(t *testing.T) {
	result := CountConversations(nil)

	if len(result) != 0 {
		t.Fatalf("expected 0 conversations, got %d", len(result))
	}
}
