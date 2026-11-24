package query

import (
	"net/netip"

	"github.com/akshya1996/illumio-assignment/internal/model"
)

// Conversation identifies one network conversation: traffic from a source
// IP and port to a destination IP and port over one protocol. Records with
// the same values for all of these belong to the same conversation.
//
// Direction matters: A→B and B→A are two different conversations.
//
// Conversation is used as a map key, so it holds plain values instead of
// pointers. Two pointers to the same address would still compare as
// different keys, which would split one conversation into many. Because a
// FlowRecord's IPs and ports can be missing, each one has a matching
// ...Present flag. This keeps "no port" (SourcePortPresent false) separate
// from a real port 0 (SourcePortPresent true, SourcePort 0). When a Present
// flag is false, its value field is left at its zero value.
type Conversation struct {
	SourceIP               netip.Addr
	SourceIPPresent        bool
	SourcePort             int
	SourcePortPresent      bool
	DestinationIP          netip.Addr
	DestinationIPPresent   bool
	DestinationPort        int
	DestinationPortPresent bool
	Protocol               int
}

// CountConversations groups records by Conversation and returns how many
// records belong to each one. It counts records (flow log lines), not
// packets or bytes. Empty or nil input returns an empty, non-nil map.
func CountConversations(records []model.FlowRecord) map[Conversation]int {
	counts := make(map[Conversation]int)

	for _, record := range records {
		// Protocol is always present on a record, so copy it directly
		conversation := Conversation{
			Protocol: record.Protocol,
		}

		// Copy each optional field only if the record has it, and mark it
		// as present. Missing fields stay at zero with Present = false.
		if record.SourceIP != nil {
			conversation.SourceIP = *record.SourceIP
			conversation.SourceIPPresent = true
		}

		if record.DestinationIP != nil {
			conversation.DestinationIP = *record.DestinationIP
			conversation.DestinationIPPresent = true
		}

		if record.SourcePort != nil {
			conversation.SourcePort = *record.SourcePort
			conversation.SourcePortPresent = true
		}

		if record.DestinationPort != nil {
			conversation.DestinationPort = *record.DestinationPort
			conversation.DestinationPortPresent = true
		}

		counts[conversation]++
	}

	return counts
}
