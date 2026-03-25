.PHONY: setup test clean help

## setup  : Install dependencies and run the network simulation
setup:
	@echo "--- Installing Dependencies ---"
	go mod tidy
	@echo "--- Running Network Setup ---"
	sudo go run main.go

## test   : Ping from ns1 to ns2 (cross-network test)
test:
	@echo "--- Testing Ping: ns1 → ns2 ---"
	sudo ip netns exec ns1 ping -c 3 10.0.2.10
	@echo ""
	@echo "--- Testing Ping: ns2 → ns1 ---"
	sudo ip netns exec ns2 ping -c 3 10.0.1.10

## clean  : Tear down all namespaces and bridges
clean:
	@echo "--- Tearing Down Network ---"
	sudo ip netns del ns1       2>/dev/null || true
	sudo ip netns del ns2       2>/dev/null || true
	sudo ip netns del router-ns 2>/dev/null || true
	sudo ip link del br0        2>/dev/null || true
	sudo ip link del br1        2>/dev/null || true
	@echo "--- Done ---"

## help   : Show available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
