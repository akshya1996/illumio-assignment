// Package parser reads flow log text (in the style of AWS VPC Flow Logs)
// and turns each line into a model.FlowRecord.
//
// The input must start with a header line that names the columns, for
// example:
//
//	version account-id interface-id srcaddr dstaddr srcport dstport protocol packets bytes start end action log-status
//
// Columns are matched by name rather than position, so they can appear in
// any order and extra columns are ignored. Fields are separated by
// whitespace, and "-" means the value is missing.
package parser

import (
	"bufio"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"

	"github.com/akshya1996/illumio-assignment/internal/model"
)

// requiredFields lists the columns the header must contain. If any of them
// is missing, parsing stops before reading any records.
var requiredFields = []string{
	"srcaddr",
	"dstaddr",
	// "srcport", optional, can be missing
	// "dstport", optional, can be missing
	"protocol",
	"packets",
	"bytes",
	"start",
	"end",
	"action",
}

// getField returns the value of the column called name from one record's
// fields. It returns an error if the header has no such column, or if the
// record is too short to contain it.
func getField(fields []string, fieldMap map[string]int, name string) (string, error) {
	index, exists := fieldMap[name]
	if !exists {
		return "", fmt.Errorf("missing field in header: %s", name)
	}

	if index >= len(fields) {
		return "", fmt.Errorf("field %s is missing from record", name)
	}

	return fields[index], nil
}

// validateHeader checks that all required fields are present in the header.
// It returns an error listing every missing field.
func validateHeader(fieldMap map[string]int) error {
	var missingFields []string

	for _, field := range requiredFields {
		if _, exists := fieldMap[field]; !exists {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("required fields missing from header: %s",
			strings.Join(missingFields, ", "))
	}

	return nil
}

// buildFieldMap maps each column name in the header to its position, so
// later code can look up fields by name. A column name that appears twice
// is an error, since it would be unclear which one to use.
func buildFieldMap(header []string) (map[string]int, error) {
	fieldMap := make(map[string]int)

	for i, field := range header {
		if _, exists := fieldMap[field]; exists {
			return nil, fmt.Errorf("duplicate field in header: %s", field)
		}

		fieldMap[field] = i
	}

	return fieldMap, nil
}

// Parse reads flow log data from r and returns the records it could parse,
// along with any errors it hit.
//
// Problems with the header (empty input, duplicate or missing required
// columns) stop parsing straight away, so no records are returned. Problems
// with a single line (wrong number of fields, bad values, unknown
// log-status) are recorded as an error that includes the line number, and
// parsing carries on with the next line. This means one bad line does not
// throw away the rest of the file.
//
// Blank lines are ignored. Lines whose log-status is NODATA or SKIPDATA are
// skipped without an error, since they do not describe real traffic.
func Parse(r io.Reader) ([]model.FlowRecord, []error) {
	var records []model.FlowRecord
	var parseErrors []error

	scanner := bufio.NewScanner(r)

	// Read the header. If there is no first line, either the read failed or
	// the input was empty; report whichever it was.
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			parseErrors = append(parseErrors, err)
		} else {
			parseErrors = append(parseErrors, fmt.Errorf("empty input: missing header"))
		}
		return records, parseErrors
	}

	header := strings.Fields(scanner.Text())

	if len(header) == 0 {
		parseErrors = append(parseErrors, fmt.Errorf("empty header"))
		return records, parseErrors
	}

	// Build the column name -> position mapping
	fieldMap, err := buildFieldMap(header)
	if err != nil {
		parseErrors = append(parseErrors, err)
		return records, parseErrors
	}

	// Make sure all required columns are present before reading any records
	if err := validateHeader(fieldMap); err != nil {
		parseErrors = append(parseErrors, err)
		return records, parseErrors
	}

	// Read and parse records. lineNumber starts at 1 because the header was
	// line 1; it is used to point at the bad line in error messages.
	lineNumber := 1

	for scanner.Scan() {
		lineNumber++

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Every record must have exactly one value per header column,
		// otherwise values could end up matched to the wrong column.
		fields := strings.Fields(line)
		if len(fields) != len(header) {
			parseErrors = append(
				parseErrors,
				fmt.Errorf(
					"line %d: expected %d fields, got %d",
					lineNumber,
					len(header),
					len(fields),
				),
			)
			continue
		}

		// Check log status: skip lines that carry no traffic data
		skip, err := shouldSkipRecord(fields, fieldMap)
		if err != nil {
			parseErrors = append(
				parseErrors,
				fmt.Errorf("line %d: %w", lineNumber, err),
			)
			continue
		}

		if skip {
			continue
		}

		record, err := parseRecord(fields, fieldMap)
		if err != nil {
			parseErrors = append(
				parseErrors,
				fmt.Errorf("line %d: %w", lineNumber, err),
			)
			continue
		}

		records = append(records, record)
	}

	// Catch read errors that ended the loop early (for example, a line
	// longer than the scanner's buffer).
	if err := scanner.Err(); err != nil {
		parseErrors = append(parseErrors, err)
	}

	return records, parseErrors
}

// parseRecord converts one line's fields into a FlowRecord. It stops at the
// first invalid value and returns an error describing it.
//
// IP addresses and ports given as "-" are left as nil. All other fields are
// required and must be valid numbers, except action, which is copied as is.
func parseRecord(fields []string, fieldMap map[string]int) (model.FlowRecord, error) {
	var record model.FlowRecord

	// Parse source IP ("-" leaves it nil)
	srcIP, err := getField(fields, fieldMap, "srcaddr")
	if err != nil {
		return record, err
	}

	if srcIP != "-" {
		parsedIP, err := netip.ParseAddr(srcIP)
		if err != nil {
			return record, fmt.Errorf("invalid source IP %q: %w", srcIP, err)
		}
		record.SourceIP = &parsedIP
	}

	// Parse destination IP ("-" leaves it nil)
	dstIP, err := getField(fields, fieldMap, "dstaddr")
	if err != nil {
		return record, err
	}

	if dstIP != "-" {
		parsedIP, err := netip.ParseAddr(dstIP)
		if err != nil {
			return record, fmt.Errorf("invalid destination IP %q: %w", dstIP, err)
		}
		record.DestinationIP = &parsedIP
	}

	// Parse source port (optional column)
	record.SourcePort, err = parseOptionalPort(fields, fieldMap, "srcport")
	if err != nil {
		return record, fmt.Errorf("source port: %w", err)
	}

	// Parse destination port (optional column)
	record.DestinationPort, err = parseOptionalPort(fields, fieldMap, "dstport")
	if err != nil {
		return record, fmt.Errorf("destination port: %w", err)
	}

	// Parse packets
	record.Packets, err = parseInt64Field(fields, fieldMap, "packets")
	if err != nil {
		return record, err
	}

	// Parse bytes
	record.Bytes, err = parseInt64Field(fields, fieldMap, "bytes")
	if err != nil {
		return record, err
	}

	// Parse start time (Unix seconds)
	record.StartTime, err = parseInt64Field(fields, fieldMap, "start")
	if err != nil {
		return record, err
	}

	// Parse end time (Unix seconds)
	record.EndTime, err = parseInt64Field(fields, fieldMap, "end")
	if err != nil {
		return record, err
	}

	// Parse protocol (IANA protocol number, e.g. 6 = TCP)
	protocol, err := getField(fields, fieldMap, "protocol")
	if err != nil {
		return record, err
	}

	record.Protocol, err = strconv.Atoi(protocol)
	if err != nil {
		return record, fmt.Errorf("invalid protocol %q: %w", protocol, err)
	}

	// Parse action. It is stored exactly as written (e.g. "ACCEPT"),
	// without checking or changing its value.
	record.Action, err = getField(fields, fieldMap, "action")
	if err != nil {
		return record, err
	}

	return record, nil
}

// parseOptionalPort reads a port column that may not exist. If the header
// has no column called name, it returns nil with no error. Otherwise it
// parses the value with parsePort.
func parseOptionalPort(fields []string, fieldMap map[string]int, name string) (*int, error) {
	// Check whether the port field exists in the header
	if _, exists := fieldMap[name]; !exists {
		return nil, nil
	}

	value, err := getField(fields, fieldMap, name)
	if err != nil {
		return nil, err
	}

	return parsePort(value)
}

// parsePort converts a port string to a number. "-" returns nil (no port).
// Anything else must be a whole number from 0 to 65535.
func parsePort(value string) (*int, error) {
	if value == "-" {
		return nil, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("invalid port %q: %w", value, err)
	}

	if port < 0 || port > 65535 {
		return nil, fmt.Errorf("port out of range: %d", port)
	}

	return &port, nil
}

// parseInt64Field reads the column called name and parses it as a base-10
// int64. The error message includes the column name and the bad value.
func parseInt64Field(fields []string, fieldMap map[string]int, name string) (int64, error) {
	value, err := getField(fields, fieldMap, name)
	if err != nil {
		return 0, err
	}

	result, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", name, value, err)
	}

	return result, nil
}

// shouldSkipRecord decides from the log-status column whether a line should
// be left out of the results:
//
//   - No log-status column: keep every line.
//   - "OK": keep the line.
//   - "NODATA" (no traffic in the time window) or "SKIPDATA" (some records
//     were dropped during capture): skip the line, since its traffic fields
//     are not real data.
//   - Any other value: return an error.
func shouldSkipRecord(fields []string, fieldMap map[string]int) (bool, error) {
	if _, exists := fieldMap["log-status"]; !exists {
		return false, nil
	}

	status, err := getField(fields, fieldMap, "log-status")
	if err != nil {
		return false, err
	}

	switch status {
	case "OK":
		return false, nil
	case "NODATA", "SKIPDATA":
		return true, nil
	default:
		return false, fmt.Errorf("unknown log status: %s", status)
	}
}
