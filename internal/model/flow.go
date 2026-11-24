// Package model defines the core data types shared across the parser and
// query packages.
package model

import "net/netip"

// FlowRecord is one parsed network flow log entry (in the style of AWS VPC
// Flow Logs). Each field maps to a column in the log header; see
// internal/parser for how the columns are read.
//
// Fields that may be absent in the source data are pointers: a nil value
// means the log contained "-" (no data) or the column was not in the header
// at all, which is different from a real zero value.
type FlowRecord struct {
	// SourceIP is the address the traffic came from ("srcaddr").
	// It can hold IPv4 or IPv6. Nil if the log recorded "-".
	SourceIP *netip.Addr

	// DestinationIP is the address the traffic went to ("dstaddr").
	// It can hold IPv4 or IPv6. Nil if the log recorded "-".
	DestinationIP *netip.Addr

	// SourcePort is the sending port ("srcport"), in the range 0-65535.
	// Nil if the value was "-" or the column is missing from the header.
	SourcePort *int

	// DestinationPort is the receiving port ("dstport"), in the range 0-65535.
	// Nil if the value was "-" or the column is missing from the header.
	DestinationPort *int

	// Protocol is the IANA protocol number ("protocol"),
	// e.g. 6 = TCP, 17 = UDP, 1 = ICMP.
	Protocol int

	// Packets is the number of packets sent during the flow ("packets").
	Packets int64

	// Bytes is the number of bytes sent during the flow ("bytes").
	Bytes int64

	// StartTime is when the first packet of the flow was seen ("start"),
	// as Unix time in seconds.
	StartTime int64

	// EndTime is when the last packet of the flow was seen ("end"),
	// as Unix time in seconds.
	EndTime int64

	// Action is whether the traffic was allowed or blocked ("action"),
	// typically "ACCEPT" or "REJECT". It is stored exactly as it appears
	// in the log, without changing its case.
	Action string
}
