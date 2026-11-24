package main

import (
	"fmt"
	"math/rand"
	"os"
)

const seed int64 = 42

func main() {
	file, err := os.Create("samples/generated_flow.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating sample file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	rng := rand.New(rand.NewSource(seed))

	_, err = fmt.Fprintln(file,
		"version account-id interface-id srcaddr dstaddr srcport dstport protocol packets bytes start end action log-status",
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "writing header: %v\n", err)
		os.Exit(1)
	}

	sourceIPs := []string{
		"10.0.0.1",
		"10.0.0.2",
		"10.0.0.3",
	}
	destinationIPs := []string{
		"10.0.1.1",
		"10.0.1.2",
		"10.0.1.3",
	}
	ports := []int{80, 443, 8080}
	protocols := []int{6, 17}
	actions := []string{"ACCEPT", "REJECT"}

	for i := 0; i < 100; i++ {
		srcIP := sourceIPs[rng.Intn(len(sourceIPs))]
		dstIP := destinationIPs[rng.Intn(len(destinationIPs))]
		srcPort := 50000 + rng.Intn(1000)
		dstPort := ports[rng.Intn(len(ports))]
		protocol := protocols[rng.Intn(len(protocols))]
		packets := 1 + rng.Intn(100)
		bytes := packets * (50 + rng.Intn(100))
		start := 1000 + rng.Intn(3600)
		end := start + 1 + rng.Intn(300)
		action := actions[rng.Intn(len(actions))]

		_, err := fmt.Fprintf(
			file,
			"2 123456789 eni-%03d %s %s %d %d %d %d %d %d %d %s OK\n",
			i,
			srcIP,
			dstIP,
			srcPort,
			dstPort,
			protocol,
			packets,
			bytes,
			start,
			end,
			action,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "writing record: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Generated 100 flow records using seed 42")
}
