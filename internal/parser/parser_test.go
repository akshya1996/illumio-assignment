// Tests for Parse. Each test builds a small flow log as a string (a header
// line followed by one or more records) and checks the records and errors
// that come back.
//
// The tests fall into three groups:
//   - Valid input: IPv4, IPv6, columns in a different order, optional and
//     "-" values.
//   - Line-level errors: a bad line is reported and skipped, and parsing
//     carries on.
//   - Header-level errors: parsing stops and no records are returned.

package parser

import (
	"strings"
	"testing"
)

// TestParseIPv4Record checks that a simple valid IPv4 record is parsed and
// its source IP is read correctly.
func TestParseIPv4Record(t *testing.T) {
	// The record line starts with a tab. This also checks that leading
	// whitespace on a line is ignored.
	input := `version srcaddr dstaddr protocol packets bytes start end action
	2 10.0.0.1 10.0.0.2 6 10 500 1700000000 1700000060 ACCEPT`
	records, errs := Parse(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].SourceIP == nil {
		t.Fatal("expected source IP, got nil")
	}

	if got := records[0].SourceIP.String(); got != "10.0.0.1" {
		t.Errorf("expected source IP 10.0.0.1, got %s", got)
	}
}

// TestParseIPv6Record checks that IPv6 source and destination addresses are
// parsed and printed back in their normal short form.
func TestParseIPv6Record(t *testing.T) {
	input := `version srcaddr dstaddr protocol packets bytes start end action
2 2001:db8::1 2001:db8::2 6 10 500 1700000000 1700000060 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].SourceIP == nil {
		t.Fatal("expected source IP, got nil")
	}

	if got := records[0].SourceIP.String(); got != "2001:db8::1" {
		t.Errorf("expected source IP 2001:db8::1, got %s", got)
	}

	if records[0].DestinationIP == nil {
		t.Fatal("expected destination IP, got nil")
	}

	if got := records[0].DestinationIP.String(); got != "2001:db8::2" {
		t.Errorf("expected destination IP 2001:db8::2, got %s", got)
	}
}

// TestParseDifferentHeaderOrder checks that columns are matched by name, not
// position. The header swaps srcaddr/dstaddr, bytes/packets and start/end
// compared with the usual order, and the values must still land in the
// right fields.
func TestParseDifferentHeaderOrder(t *testing.T) {
	input := `version dstaddr srcaddr bytes packets protocol end start action
2 10.0.0.2 10.0.0.1 500 10 6 1700000060 1700000000 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if got := records[0].SourceIP.String(); got != "10.0.0.1" {
		t.Errorf("expected source IP 10.0.0.1, got %s", got)
	}

	if got := records[0].Bytes; got != 500 {
		t.Errorf("expected 500 bytes, got %d", got)
	}

	if got := records[0].Packets; got != 10 {
		t.Errorf("expected 10 packets, got %d", got)
	}
}

// TestParseMissingOptionalPorts checks that srcport and dstport can be left
// out of the header entirely. The record should still parse, with both
// ports set to nil.
func TestParseMissingOptionalPorts(t *testing.T) {
	input := `version srcaddr dstaddr protocol packets bytes start end action
2 10.0.0.1 10.0.0.2 6 10 500 1700000000 1700000060 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].SourcePort != nil {
		t.Errorf("expected source port to be nil, got %v", *records[0].SourcePort)
	}

	if records[0].DestinationPort != nil {
		t.Errorf("expected destination port to be nil, got %v", *records[0].DestinationPort)
	}
}

// TestParseDashPlaceholders checks that "-" in an IP or port column is read
// as "no value" (nil), while the other columns on the same line are still
// parsed normally.
func TestParseDashPlaceholders(t *testing.T) {
	// Source IP and source port are "-"; destination IP and port are real.
	input := `version srcaddr dstaddr srcport dstport protocol packets bytes start end action
2 - 10.0.0.2 - 443 6 10 500 1700000000 1700000060 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	record := records[0]

	if record.SourceIP != nil {
		t.Errorf("expected source IP to be nil")
	}

	if record.DestinationIP == nil {
		t.Fatal("expected destination IP, got nil")
	}

	if record.SourcePort != nil {
		t.Errorf("expected source port to be nil")
	}

	if record.DestinationPort == nil {
		t.Fatal("expected destination port, got nil")
	}

	if got := *record.DestinationPort; got != 443 {
		t.Errorf("expected destination port 443, got %d", got)
	}
}

// TestParseInvalidIP checks that a record with a malformed IP address is
// rejected with exactly one error and is not returned as a record.
func TestParseInvalidIP(t *testing.T) {
	input := `version srcaddr dstaddr protocol packets bytes start end action
2 invalid-ip 10.0.0.2 6 10 500 1700000000 1700000060 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

// TestParseContinuesAfterInvalidRecord checks that one bad line does not
// stop parsing. The first record has an invalid IP and the second is valid,
// so we expect one error and one record (the second line).
func TestParseContinuesAfterInvalidRecord(t *testing.T) {
	input := `version srcaddr dstaddr protocol packets bytes start end action
2 invalid-ip 10.0.0.2 6 10 500 1700000000 1700000060 ACCEPT
2 10.0.0.1 10.0.0.2 6 10 500 1700000000 1700000060 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 valid record, got %d", len(records))
	}

	if got := records[0].SourceIP.String(); got != "10.0.0.1" {
		t.Errorf("expected source IP 10.0.0.1, got %s", got)
	}
}

// TestParseSkipsNonTrafficRecords checks the log-status column. The OK line
// is kept, and the NODATA and SKIPDATA lines are dropped without an error.
// Those lines use "-" for protocol, packets and so on, which would fail to
// parse. This shows they are skipped before their fields are parsed.
func TestParseSkipsNonTrafficRecords(t *testing.T) {
	input := `version srcaddr dstaddr protocol packets bytes start end action log-status
2 10.0.0.1 10.0.0.2 6 10 500 1700000000 1700000060 ACCEPT OK
2 - - - - - 1700000000 1700000060 - NODATA
2 - - - - - 1700000000 1700000060 - SKIPDATA`

	records, errs := Parse(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 traffic record, got %d", len(records))
	}

	if got := records[0].Action; got != "ACCEPT" {
		t.Errorf("expected action ACCEPT, got %s", got)
	}
}

// TestParseDuplicateHeaders checks that a header naming the same column
// twice (srcaddr here) is a header-level error: parsing stops, no records
// are returned, and the error names the duplicate column.
func TestParseDuplicateHeaders(t *testing.T) {
	input := `version srcaddr dstaddr srcaddr protocol packets bytes start end action
2 10.0.0.1 10.0.0.2 10.0.0.3 6 10 500 1700000000 1700000060 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	expectedError := "duplicate field in header: srcaddr"

	if !strings.Contains(errs[0].Error(), expectedError) {
		t.Errorf("expected error containing %q, got %q",
			expectedError, errs[0].Error())
	}
}

// TestParseIncorrectFieldCount checks that a record with fewer values than
// the header has columns is rejected, and that the error gives both counts.
func TestParseIncorrectFieldCount(t *testing.T) {
	// The header has 9 columns, but the record is missing the "end" value,
	// so it only has 8.
	input := `version srcaddr dstaddr protocol packets bytes start end action
2 10.0.0.1 10.0.0.2 6 10 500 1700000000 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	expectedError := "expected 9 fields, got 8"

	if !strings.Contains(errs[0].Error(), expectedError) {
		t.Errorf("expected error containing %q, got %q",
			expectedError, errs[0].Error())
	}
}

// TestParseInvalidProtocol checks that protocol must be a number. A name
// like "TCP" is rejected, and the error quotes the bad value.
func TestParseInvalidProtocol(t *testing.T) {
	input := `version srcaddr dstaddr protocol packets bytes start end action
2 10.0.0.1 10.0.0.2 TCP 10 500 1700000000 1700000060 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	expectedError := `invalid protocol "TCP"`

	if !strings.Contains(errs[0].Error(), expectedError) {
		t.Errorf("expected error containing %q, got %q",
			expectedError, errs[0].Error())
	}
}

// TestParseInvalidPackets checks that a non-numeric packets value is
// rejected, and the error names the column and the bad value.
func TestParseInvalidPackets(t *testing.T) {
	input := `version srcaddr dstaddr protocol packets bytes start end action
2 10.0.0.1 10.0.0.2 6 abc 500 1700000000 1700000060 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	expectedError := `invalid packets value "abc"`

	if !strings.Contains(errs[0].Error(), expectedError) {
		t.Errorf("expected error containing %q, got %q",
			expectedError, errs[0].Error())
	}
}

// TestParsePortOutOfRange checks that a port above 65535 (here 70000 as the
// source port) is rejected, even though it is a valid number.
func TestParsePortOutOfRange(t *testing.T) {
	input := `version srcaddr dstaddr srcport dstport protocol packets bytes start end action
2 10.0.0.1 10.0.0.2 70000 443 6 10 500 1700000000 1700000060 ACCEPT`

	records, errs := Parse(strings.NewReader(input))

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	expectedError := "port out of range: 70000"

	if !strings.Contains(errs[0].Error(), expectedError) {
		t.Errorf("expected error containing %q, got %q",
			expectedError, errs[0].Error())
	}
}

// TestParseUnknownLogStatus checks that a log-status other than OK, NODATA
// or SKIPDATA is an error, not silently kept or skipped.
func TestParseUnknownLogStatus(t *testing.T) {
	input := `version srcaddr dstaddr protocol packets bytes start end action log-status
2 10.0.0.1 10.0.0.2 6 10 500 1700000000 1700000060 ACCEPT UNKNOWN`

	records, errs := Parse(strings.NewReader(input))

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	expectedError := "unknown log status: UNKNOWN"

	if !strings.Contains(errs[0].Error(), expectedError) {
		t.Errorf("expected error containing %q, got %q",
			expectedError, errs[0].Error())
	}
}

// TestParseMissingRequiredHeader checks that a header missing required
// columns is a header-level error: no records are returned, even though the
// record line itself matches the header. All missing columns should be
// reported together in a single error, not just the first one found.
func TestParseMissingRequiredHeader(t *testing.T) {
	// The header is missing four required columns: dstaddr, bytes, end and
	// action.
	input := `version srcaddr protocol packets start
2 10.0.0.1 6 10 1700000000`

	records, errs := Parse(strings.NewReader(input))

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	expectedFields := []string{"dstaddr", "bytes", "end", "action"}

	for _, field := range expectedFields {
		if !strings.Contains(errs[0].Error(), field) {
			t.Errorf("expected error to contain missing field %q, got %q",
				field, errs[0].Error())
		}
	}
}
