// Tests for run, the body of the flowline command.
//
// The error tests check that bad arguments are rejected with a clear
// message. Most of them point --input at a file that does not need to
// exist, because run validates arguments before it opens the file.
//
// The end-to-end tests write a small flow log to a temporary directory,
// run a query on it, and check the printed output. They capture stdout by
// temporarily replacing os.Stdout with a pipe.

package main

import (
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/akshya1996/illumio-assignment/internal/query"
)

// TestRunMissingInput checks that --input is required.
func TestRunMissingInput(t *testing.T) {
	err := run([]string{"--query", "filter"})

	if err == nil {
		t.Fatal("expected an error when input is missing")
	}

	if !strings.Contains(err.Error(), "--input is required") {
		t.Errorf("expected missing input error, got %v", err)
	}
}

// TestRunUnsupportedQuery checks that an unknown --query name is rejected,
// and that the error quotes the name the user gave.
func TestRunUnsupportedQuery(t *testing.T) {
	err := run([]string{
		"--input", "samples/flow_v2.log",
		"--query", "invalid",
	})

	if err == nil {
		t.Fatal("expected an error for unsupported query")
	}

	if !strings.Contains(err.Error(), `unsupported query "invalid"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunMissingFile checks that an input file that does not exist gives an
// "open input file" error. The arguments are otherwise valid, so this is
// the only test here that reaches the os.Open call.
func TestRunMissingFile(t *testing.T) {
	err := run([]string{
		"--input", "samples/nonexistent.log",
		"--query", "filter",
	})

	if err == nil {
		t.Fatal("expected an error for missing input file")
	}

	if !strings.Contains(err.Error(), "open input file") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunInvalidSourceIP checks that a malformed --src-ip is rejected before
// the input file is opened.
func TestRunInvalidSourceIP(t *testing.T) {
	err := run([]string{
		"--input", "samples/flow_v2.log",
		"--query", "filter",
		"--src-ip", "invalid-ip",
	})

	if err == nil {
		t.Fatal("expected an error for invalid source IP")
	}

	if !strings.Contains(err.Error(), "invalid source IP") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunInvalidSourcePort checks that a --src-port above 65535 is rejected.
func TestRunInvalidSourcePort(t *testing.T) {
	err := run([]string{
		"--input", "samples/flow_v2.log",
		"--query", "filter",
		"--src-port", "70000",
	})

	if err == nil {
		t.Fatal("expected an error for invalid source port")
	}

	if !strings.Contains(err.Error(), "source port must be between 0 and 65535") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunFilterBySourceIP runs the filter query end to end. Only the record
// from 10.0.0.1 should be printed; the one from 10.0.0.3 should not.
func TestRunFilterBySourceIP(t *testing.T) {
	input := `version srcaddr dstaddr srcport dstport protocol packets bytes start end action
2 10.0.0.1 10.0.0.2 50000 443 6 10 500 100 150 ACCEPT
2 10.0.0.3 10.0.0.4 50001 443 6 5 300 110 160 REJECT
`

	// Write the log to a temporary directory that is deleted after the test
	inputFile := filepath.Join(t.TempDir(), "flow.log")
	if err := os.WriteFile(inputFile, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}

	// Send stdout into a pipe so the test can read what run prints
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer

	runErr := run([]string{
		"--input", inputFile,
		"--query", "filter",
		"--src-ip", "10.0.0.1",
	})

	// Close the write end so ReadAll sees end of input, then put the real
	// stdout back
	writer.Close()
	os.Stdout = oldStdout

	output, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}

	if runErr != nil {
		t.Fatalf("run returned unexpected error: %v", runErr)
	}

	if !strings.Contains(string(output), "10.0.0.1") {
		t.Errorf("expected output to contain matching source IP, got:\n%s", output)
	}

	if strings.Contains(string(output), "10.0.0.3") {
		t.Errorf("output should not contain the non-matching source IP, got:\n%s", output)
	}

	// Ports must be printed as numbers, not as pointer addresses
	want := "10.0.0.1:50000 -> 10.0.0.2:443 protocol=6 packets=10 bytes=500 start=100 end=150 action=ACCEPT"
	if !strings.Contains(string(output), want) {
		t.Errorf("expected record line %q, got:\n%s", want, output)
	}
}

// TestRunConversations runs the conversations query end to end. The first
// two lines are the same conversation (count 2). The third is the reply in
// the other direction, so it is a separate conversation (count 1).
func TestRunConversations(t *testing.T) {
	input := `version srcaddr dstaddr srcport dstport protocol packets bytes start end action
2 10.0.0.1 10.0.0.2 50000 443 6 10 500 100 150 ACCEPT
2 10.0.0.1 10.0.0.2 50000 443 6 5 300 110 160 ACCEPT
2 10.0.0.2 10.0.0.1 443 50000 6 8 400 120 170 REJECT
`

	// Write the log to a temporary directory that is deleted after the test
	inputFile := filepath.Join(t.TempDir(), "flow.log")
	if err := os.WriteFile(inputFile, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}

	// Send stdout into a pipe so the test can read what run prints
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer

	runErr := run([]string{
		"--input", inputFile,
		"--query", "conversations",
	})

	// Close the write end so ReadAll sees end of input, then put the real
	// stdout back
	writer.Close()
	os.Stdout = oldStdout

	output, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}

	if runErr != nil {
		t.Fatalf("run returned unexpected error: %v", runErr)
	}

	if !strings.Contains(string(output), "2 records") {
		t.Errorf("expected one conversation with 2 records, got:\n%s", output)
	}

	if !strings.Contains(string(output), "10.0.0.1:50000 -> 10.0.0.2:443 protocol=6: 2 records") {
		t.Errorf("expected one conversation with 2 records, got:\n%s", output)
	}

	if !strings.Contains(string(output), "10.0.0.2:443 -> 10.0.0.1:50000 protocol=6: 1 record\n") {
		t.Errorf("expected reverse conversation with 1 record, got:\n%s", output)
	}
}

// TestRunTopTalkers runs the top-talkers query end to end with window
// [100, 200) and n = 2. Expected totals:
//   - 10.0.0.1: 500 + 300 = 800 bytes
//   - 10.0.0.2: 400 bytes
//   - 10.0.0.3: its only record starts at 200, which is outside the window,
//     so it must not appear.
func TestRunTopTalkers(t *testing.T) {
	input := `version srcaddr dstaddr srcport dstport protocol packets bytes start end action
2 10.0.0.1 10.0.0.2 50000 443 6 10 500 100 150 ACCEPT
2 10.0.0.1 10.0.0.3 50001 443 6 5 300 110 160 ACCEPT
2 10.0.0.2 10.0.0.1 443 50000 6 8 400 120 170 REJECT
2 10.0.0.3 10.0.0.1 443 50000 6 8 900 200 250 ACCEPT
`

	// Write the log to a temporary directory that is deleted after the test
	inputFile := filepath.Join(t.TempDir(), "flow.log")
	if err := os.WriteFile(inputFile, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}

	// Send stdout into a pipe so the test can read what run prints
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer

	runErr := run([]string{
		"--input", inputFile,
		"--query", "top-talkers",
		"--from", "100",
		"--to", "200",
		"--n", "2",
	})

	// Close the write end so ReadAll sees end of input, then put the real
	// stdout back
	writer.Close()
	os.Stdout = oldStdout

	output, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}

	if runErr != nil {
		t.Fatalf("run returned unexpected error: %v", runErr)
	}

	if !strings.Contains(string(output), "10.0.0.1") ||
		!strings.Contains(string(output), "800") {
		t.Errorf("expected 10.0.0.1 with 800 bytes, got:\n%s", output)
	}

	if !strings.Contains(string(output), "10.0.0.2") ||
		!strings.Contains(string(output), "400") {
		t.Errorf("expected 10.0.0.2 with 400 bytes, got:\n%s", output)
	}

	if strings.Contains(string(output), "10.0.0.3") {
		t.Errorf("expected top 2 talkers only, got:\n%s", output)
	}
}

// TestFormatEndpoint checks how an IP and port are printed, including
// missing values and IPv6 brackets.
func TestFormatEndpoint(t *testing.T) {
	ipv4 := netip.MustParseAddr("10.0.0.1")
	ipv6 := netip.MustParseAddr("2001:db8::1")
	port := 443

	tests := []struct {
		name string
		ip   *netip.Addr
		port *int
		want string
	}{
		{"IPv4 with port", &ipv4, &port, "10.0.0.1:443"},
		{"IPv6 with port", &ipv6, &port, "[2001:db8::1]:443"},
		{"missing port", &ipv4, nil, "10.0.0.1:-"},
		{"missing IP", nil, &port, "-:443"},
		{"both missing", nil, nil, "-:-"},
	}

	for _, tt := range tests {
		if got := formatEndpoint(tt.ip, tt.port); got != tt.want {
			t.Errorf("%s: expected %q, got %q", tt.name, tt.want, got)
		}
	}
}

// TestCompareConversationsNumericOrder checks that conversations are sorted
// by IP as numbers, not as text: 10.0.0.2 must come before 10.0.0.10, and a
// missing port must come before port 0.
func TestCompareConversationsNumericOrder(t *testing.T) {
	ip2 := netip.MustParseAddr("10.0.0.2")
	ip10 := netip.MustParseAddr("10.0.0.10")

	conversations := []query.Conversation{
		{SourceIP: ip10, SourceIPPresent: true},
		{SourceIP: ip2, SourceIPPresent: true, SourcePort: 0, SourcePortPresent: true},
		{SourceIP: ip2, SourceIPPresent: true}, // no source port
	}

	slices.SortFunc(conversations, compareConversations)

	if conversations[0].SourceIP != ip2 || conversations[0].SourcePortPresent {
		t.Errorf("expected 10.0.0.2 with no port first, got %+v", conversations[0])
	}

	if conversations[1].SourceIP != ip2 || !conversations[1].SourcePortPresent {
		t.Errorf("expected 10.0.0.2 port 0 second, got %+v", conversations[1])
	}

	if conversations[2].SourceIP != ip10 {
		t.Errorf("expected 10.0.0.10 last, got %+v", conversations[2])
	}
}
