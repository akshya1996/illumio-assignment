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
	"cmp"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/akshya1996/illumio-assignment/internal/model"
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

	fmt.Printf("Parsed %s\n", plural(len(records), "valid record"))

	// Apply the filter for every query type. With no filter flags set this
	// keeps all records.
	filteredRecords := query.FilterRecords(records, filter)

	switch *queryType {
	case "filter":
		fmt.Printf("Matched %s:\n", plural(len(filteredRecords), "record"))

		for _, record := range filteredRecords {
			fmt.Println(formatRecord(record))
		}

	case "conversations":
		counts := query.CountConversations(filteredRecords)

		// Map order is random, so collect the keys and sort them to get the
		// same output every run.
		conversations := make([]query.Conversation, 0, len(counts))
		for conversation := range counts {
			conversations = append(conversations, conversation)
		}

		slices.SortFunc(conversations, compareConversations)

		fmt.Printf("Found %s:\n", plural(len(counts), "conversation"))

		for _, conversation := range conversations {
			fmt.Printf("%s: %s\n",
				formatConversation(conversation),
				plural(counts[conversation], "record"))
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

// formatRecord prints a record on one line, for example:
//
//	10.0.0.1:50000 -> 10.0.0.2:443 protocol=6 packets=10 bytes=500 start=100 end=150 action=ACCEPT
func formatRecord(record model.FlowRecord) string {
	return fmt.Sprintf("%s -> %s protocol=%d packets=%d bytes=%d start=%d end=%d action=%s",
		formatEndpoint(record.SourceIP, record.SourcePort),
		formatEndpoint(record.DestinationIP, record.DestinationPort),
		record.Protocol,
		record.Packets,
		record.Bytes,
		record.StartTime,
		record.EndTime,
		record.Action,
	)
}

// formatConversation prints a conversation key on one line, for example:
//
//	10.0.0.1:50000 -> 10.0.0.2:443 protocol=6
func formatConversation(c query.Conversation) string {
	var srcIP, dstIP *netip.Addr
	var srcPort, dstPort *int

	// Turn the Present flags back into nil/non-nil so formatEndpoint can
	// print missing values as "-".
	if c.SourceIPPresent {
		srcIP = &c.SourceIP
	}
	if c.SourcePortPresent {
		srcPort = &c.SourcePort
	}
	if c.DestinationIPPresent {
		dstIP = &c.DestinationIP
	}
	if c.DestinationPortPresent {
		dstPort = &c.DestinationPort
	}

	return fmt.Sprintf("%s -> %s protocol=%d",
		formatEndpoint(srcIP, srcPort),
		formatEndpoint(dstIP, dstPort),
		c.Protocol,
	)
}

// formatEndpoint prints an IP and port as "ip:port". A missing IP or port
// is printed as "-". IPv6 addresses are wrapped in brackets, as in
// "[2001:db8::1]:443", so the port is not confused with the address.
func formatEndpoint(ip *netip.Addr, port *int) string {
	host := "-"
	if ip != nil {
		host = ip.String()
		if ip.Is6() {
			host = "[" + host + "]"
		}
	}

	portText := "-"
	if port != nil {
		portText = strconv.Itoa(*port)
	}

	return host + ":" + portText
}

// compareConversations orders conversations by source IP, source port,
// destination IP, destination port, then protocol. IPs and ports are
// compared as numbers, so 10.0.0.2 comes before 10.0.0.10. A missing value
// comes before any real one, so a missing port sorts before port 0.
func compareConversations(a, b query.Conversation) int {
	return cmp.Or(
		compareOptional(a.SourceIPPresent, b.SourceIPPresent, a.SourceIP.Compare(b.SourceIP)),
		compareOptional(a.SourcePortPresent, b.SourcePortPresent, cmp.Compare(a.SourcePort, b.SourcePort)),
		compareOptional(a.DestinationIPPresent, b.DestinationIPPresent, a.DestinationIP.Compare(b.DestinationIP)),
		compareOptional(a.DestinationPortPresent, b.DestinationPortPresent, cmp.Compare(a.DestinationPort, b.DestinationPort)),
		cmp.Compare(a.Protocol, b.Protocol),
	)
}

// compareOptional compares two optional values. If only one is present, the
// missing one comes first. Otherwise it returns valueResult, the result of
// comparing the values themselves.
func compareOptional(aPresent, bPresent bool, valueResult int) int {
	if aPresent != bPresent {
		if !aPresent {
			return -1
		}
		return 1
	}

	return valueResult
}

// plural returns the count followed by word, adding "s" unless the count
// is 1 (e.g. "1 record", "2 records").
func plural(count int, word string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, word)
	}

	return fmt.Sprintf("%d %ss", count, word)
}
