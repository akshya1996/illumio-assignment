# Run `make demo` to build, test and run a demo in one step.

BINARY := ./flowline

.PHONY: demo build test generate clean

# Build, test, regenerate the sample data, then run every query on it.
demo: build test generate
	@echo
	@echo "=== 1. Filter: records from 10.0.0.1 (samples/flow_v2.log)"
	$(BINARY) --input samples/flow_v2.log --query filter --src-ip 10.0.0.1
	@echo
	@echo "=== 2. Conversations (samples/flow_v2.log)"
	$(BINARY) --input samples/flow_v2.log --query conversations
	@echo
	@echo "=== 3. Top 2 talkers in [100, 200) (samples/flow_v2.log)"
	$(BINARY) --input samples/flow_v2.log --query top-talkers --from 100 --to 200 --n 2
	@echo
	@echo "=== 4. Same queries on a different header layout (samples/flow_v3.log)"
	$(BINARY) --input samples/flow_v3.log --query filter --src-ip 10.0.1.1
	$(BINARY) --input samples/flow_v3.log --query conversations
	$(BINARY) --input samples/flow_v3.log --query top-talkers --from 200 --to 300 --n 2
	@echo
	@echo "=== 5. Filters apply to other queries: top talkers among REJECTed traffic (generated data)"
	$(BINARY) --input samples/generated_flow.log --query top-talkers --from 1000 --to 4600 --n 3 --action REJECT

build:
	go build -o $(BINARY) ./cmd/flowline

test:
	go vet ./...
	go test ./...

# Regenerate samples/generated_flow.log. The seed is fixed, so the file is
# the same every time.
generate:
	go run ./cmd/samplegen

clean:
	rm -f $(BINARY)
