// Command flowline reads a flow log file and runs one query on it.
//
// Usage:
//
//	flowline --input FILE [--query filter|conversations|top-talkers] [filter flags]
//
// Queries:
//   - filter (default): print every record that matches the filter flags.
//   - conversations: count records per conversation (source IP/port,
//     destination IP/port, protocol).
//   - top-talkers: list the --n source IPs that sent the most bytes in the
//     time window [--from, --to). Both bounds are Unix seconds.
//
// The filter flags (--src-ip, --dst-ip, --src-port, --dst-port, --protocol,
// --action) work with every query: records are filtered first, then the
// query runs on what is left.
//
// Lines that fail to parse are printed to stderr as warnings and skipped;
// they do not stop the program.
package main

import (
	"flag"
	"fmt"
	"net/netip"
	"os"
	"sort"
	"strings"

	"github.com/akshya1996/illumio-assignment/internal/parser"
	"github.com/akshya1996/illumio-assignment/internal/query"
)

// main runs the program and exits with status 1 if run returns an error.
// All the work happens in run so that tests can call it directly.
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// run parses args (the command-line arguments without the program name),
// validates them, reads the input file and prints the query result to
// stdout. It returns an error for bad arguments or a file that cannot be
// opened. Bad lines in the file are only warnings and do not cause an error.
func run(args []string) error {
	// Use our own FlagSet with ContinueOnError so a bad flag returns an
	// error instead of exiting the process, which lets tests check it.
	flags := flag.NewFlagSet("flowline", flag.ContinueOnError)

	input := flags.String("input", "", "Path to flow log file")
	queryType := flags.String("query", "filter", "Query: filter, conversations, or top-talkers")

	srcIP := flags.String("src-ip", "", "Filter by source IP")
	dstIP := flags.String("dst-ip", "", "Filter by destination IP")

	// Ports and protocol default to -1, meaning "not set". 0 cannot be used
	// for this because it is a valid port and protocol number.
	srcPort := flags.Int("src-port", -1, "Filter by source port")
	dstPort := flags.Int("dst-port", -1, "Filter by destination port")

	protocol := flags.Int("protocol", -1, "Filter by protocol")
	action := flags.String("action", "", "Filter by action (ACCEPT or REJECT)")

	// Time window and count, used only by top-talkers
	from := flags.Int64("from", 0, "Start of time window")
	to := flags.Int64("to", 0, "End of time window")

	n := flags.Int("n", 10, "Number of top talkers")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *input == "" {
		return fmt.Errorf("--input is required")
	}

	// Reject unknown query names early. The empty case means "valid, do
	// nothing here"; the query actually runs in the switch at the end.
	switch *queryType {
	case "filter", "conversations", "top-talkers":
	default:
		return fmt.Errorf("unsupported query %q", *queryType)
	}

	// Validate top-talkers arguments before opening the file.
	if *queryType == "top-talkers" {
		if *from >= *to {
			return fmt.Errorf("--from must be less than --to")
		}
		if *n <= 0 {
			return fmt.Errorf("--n must be greater than 0")
		}
	}

	// Build and validate filters before file I/O. Only flags the user set
	// are added to the filter; the rest stay nil and match anything.
	filter := query.Filter{}

	if *srcIP != "" {
		ip, err := netip.ParseAddr(*srcIP)
		if err != nil {
			return fmt.Errorf("invalid source IP: %w", err)
		}
		filter.SourceIP = &ip
	}

	if *dstIP != "" {
		ip, err := netip.ParseAddr(*dstIP)
		if err != nil {
			return fmt.Errorf("invalid destination IP: %w", err)
		}
		filter.DestinationIP = &ip
	}

	if *srcPort != -1 {
		if *srcPort < 0 || *srcPort > 65535 {
			return fmt.Errorf("source port must be between 0 and 65535")
		}
		filter.SourcePort = srcPort
	}

	if *dstPort != -1 {
		if *dstPort < 0 || *dstPort > 65535 {
			return fmt.Errorf("destination port must be between 0 and 65535")
		}
		filter.DestinationPort = dstPort
	}

	if *protocol != -1 {
		if *protocol < 0 {
			return fmt.Errorf("protocol must be non-negative")
		}
		filter.Protocol = protocol
	}

	if *action != "" {
		// Accept any case on the command line (e.g. "accept") and convert
		// to uppercase to match how actions appear in the log.
		normalizedAction := strings.ToUpper(*action)
		if normalizedAction != "ACCEPT" && normalizedAction != "REJECT" {
			return fmt.Errorf("action must be ACCEPT or REJECT")
		}
		filter.Action = &normalizedAction
	}

	// Open and parse the input file.
	file, err := os.Open(*input)
	if err != nil {
		return fmt.Errorf("open input file: %w", err)
	}
	defer file.Close()

	records, parseErrors := parser.Parse(file)

	// Report bad lines but keep going with the records that did parse
	for _, parseErr := range parseErrors {
		fmt.Fprintln(os.Stderr, "Warning:", parseErr)
	}

	fmt.Printf("Parsed %d valid records\n", len(records))

	// Apply the filter for every query type. With no filter flags set this
	// keeps all records.
	filteredRecords := query.FilterRecords(records, filter)

	switch *queryType {
	case "filter":
		fmt.Printf("Matched %d records:\n", len(filteredRecords))

		for _, record := range filteredRecords {
			fmt.Printf("%+v\n", record)
		}

	case "conversations":
		counts := query.CountConversations(filteredRecords)

		// Map order is random, so collect the keys and sort them to get the
		// same output every run. Sorting by the printed text is simple and
		// stable, but it is text order, not numeric (e.g. "10.0.0.10"
		// sorts before "10.0.0.2").

		conversations := make([]query.Conversation, 0, len(counts))
		for conversation := range counts {
			conversations = append(conversations, conversation)
		}

		sort.Slice(conversations, func(i, j int) bool {
			return fmt.Sprint(conversations[i]) < fmt.Sprint(conversations[j])
		})

		fmt.Printf("Found %d conversations:\n", len(counts))

		for _, conversation := range conversations {
			fmt.Printf("%+v: %d records\n", conversation, counts[conversation])
		}

	case "top-talkers":
		// TopTalkers returns talkers already sorted by bytes, largest first
		talkers := query.TopTalkers(filteredRecords, *from, *to, *n)

		fmt.Println("Top talkers:")
		for _, talker := range talkers {
			fmt.Printf("%s: %d bytes\n", talker.SourceIP, talker.Bytes)
		}
	}

	return nil
}
