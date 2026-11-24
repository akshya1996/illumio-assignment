// Package query selects flow records that match given criteria.
package query

import (
	"net/netip"

	"github.com/akshya1996/illumio-assignment/internal/model"
)

// Filter describes which flow records to keep. Each field is optional: a
// nil field means "match any value", and a set field must match the
// record's value exactly. A record matches only if every set field matches
// (the conditions are combined with AND). An empty Filter matches every
// record.
type Filter struct {
	// SourceIP keeps records sent from this address. IPv4 and IPv6 are
	// compared as different addresses, so 10.0.0.1 does not match
	// ::ffff:10.0.0.1.
	SourceIP *netip.Addr

	// DestinationIP keeps records sent to this address. The same IPv4/IPv6
	// rule as SourceIP applies.
	DestinationIP *netip.Addr

	// SourcePort keeps records sent from this port.
	SourcePort *int

	// DestinationPort keeps records sent to this port.
	DestinationPort *int

	// Protocol keeps records with this IANA protocol number (e.g. 6 = TCP).
	Protocol *int

	// Action keeps records with this action, e.g. "ACCEPT" or "REJECT". The
	// comparison is case-sensitive, so "accept" does not match "ACCEPT".
	Action *string
}

// FilterRecords returns the records that match filter, in their original
// order. The input slice is not changed. If no records match, the result
// is nil.
func FilterRecords(records []model.FlowRecord, filter Filter) []model.FlowRecord {
	var result []model.FlowRecord

	for _, record := range records {
		if matchesFilter(record, filter) {
			result = append(result, record)
		}
	}

	return result
}

// matchesFilter reports whether record meets every condition set in filter.
// It returns false as soon as one condition fails.
//
// If the filter sets an IP or port but the record has no value for it
// (nil, because the log had "-" or the column was missing), the record does
// not match. Protocol and action are always present on a record, so they
// are compared directly.
func matchesFilter(record model.FlowRecord, filter Filter) bool {
	if filter.SourceIP != nil {
		if record.SourceIP == nil || *record.SourceIP != *filter.SourceIP {
			return false
		}
	}

	if filter.DestinationIP != nil {
		if record.DestinationIP == nil || *record.DestinationIP != *filter.DestinationIP {
			return false
		}
	}

	if filter.SourcePort != nil {
		if record.SourcePort == nil || *record.SourcePort != *filter.SourcePort {
			return false
		}
	}

	if filter.DestinationPort != nil {
		if record.DestinationPort == nil || *record.DestinationPort != *filter.DestinationPort {
			return false
		}
	}

	if filter.Protocol != nil && record.Protocol != *filter.Protocol {
		return false
	}

	if filter.Action != nil && record.Action != *filter.Action {
		return false
	}

	// Every set condition passed
	return true
}
