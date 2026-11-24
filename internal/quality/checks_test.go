package quality

import (
	"net/netip"
	"reflect"
	"testing"

	"github.com/akshya1996/illumio-assignment/internal/model"
)

func TestFindImpossibleValues(t *testing.T) {
	records := []model.FlowRecord{
		{Packets: 10, Bytes: 500, StartTime: 100, EndTime: 150}, // valid
		{Packets: -1, Bytes: 500, StartTime: 100, EndTime: 150}, // negative packets
		{Packets: 10, Bytes: -1, StartTime: 100, EndTime: 150},  // negative bytes
		{Packets: 10, Bytes: 500, StartTime: 200, EndTime: 150}, // start after end
	}

	got := FindImpossibleValues(records)
	want := []int{1, 2, 3}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindImpossibleValues() = %v, want %v", got, want)
	}
}

func TestFindDuplicates(t *testing.T) {
	records := []model.FlowRecord{
		{Protocol: 6, Packets: 10, Bytes: 500, StartTime: 100, EndTime: 150, Action: "ACCEPT"},
		{Protocol: 6, Packets: 10, Bytes: 500, StartTime: 100, EndTime: 150, Action: "ACCEPT"},
		{Protocol: 6, Packets: 5, Bytes: 200, StartTime: 110, EndTime: 160, Action: "ACCEPT"},
		{Protocol: 6, Packets: 10, Bytes: 500, StartTime: 100, EndTime: 150, Action: "ACCEPT"},
	}

	got := FindDuplicates(records)
	want := []int{1, 3}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindDuplicates() = %v, want %v", got, want)
	}
}

func TestFindOneSidedTCP(t *testing.T) {
	ipA := netip.MustParseAddr("10.0.0.1")
	ipB := netip.MustParseAddr("10.0.0.2")

	portA := 50000
	portB := 443

	records := []model.FlowRecord{
		// Forward flow: has a reverse flow.
		{
			SourceIP: &ipA, DestinationIP: &ipB,
			SourcePort: &portA, DestinationPort: &portB,
			Protocol: 6,
		},
		// Reverse flow.
		{
			SourceIP: &ipB, DestinationIP: &ipA,
			SourcePort: &portB, DestinationPort: &portA,
			Protocol: 6,
		},
		// One-sided TCP flow.
		{
			SourceIP: &ipA, DestinationIP: &ipB,
			SourcePort: &portA, DestinationPort: &portA,
			Protocol: 6,
		},
		// UDP flow: should be ignored.
		{
			SourceIP: &ipA, DestinationIP: &ipB,
			SourcePort: &portA, DestinationPort: &portB,
			Protocol: 17,
		},
	}

	got := FindOneSidedTCP(records)
	want := []int{2}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindOneSidedTCP() = %v, want %v", got, want)
	}
}

func TestFindTrafficSpikes(t *testing.T) {
	ip := netip.MustParseAddr("10.0.0.1")

	records := []model.FlowRecord{
		{SourceIP: &ip, Bytes: 100, StartTime: 0},
		{SourceIP: &ip, Bytes: 100, StartTime: 60},
		{SourceIP: &ip, Bytes: 100, StartTime: 120},
		{SourceIP: &ip, Bytes: 100, StartTime: 180},
		{SourceIP: &ip, Bytes: 500, StartTime: 240},
	}

	got := FindTrafficSpikes(records)
	want := []int{4}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindTrafficSpikes() = %v, want %v", got, want)
	}
}
