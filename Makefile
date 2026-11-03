CLUSTER := kai-lab
CONTEXT := kind-$(CLUSTER)

.PHONY: build run test lab lab-down lab-reset

build:
	go build -o bin/kai ./cmd/kai

run:
	go run ./cmd/kai

test:
	go test ./...

# Safe to run repeatedly: creates the cluster only if it doesn't exist.
lab:
	@if kind get clusters | grep -qx '$(CLUSTER)'; then \
		echo "Cluster $(CLUSTER) already exists, skipping create"; \
	else \
		kind create cluster --name $(CLUSTER) --config lab/kind.yaml; \
	fi
	kubectl --context $(CONTEXT) wait --for=condition=Ready nodes --all --timeout=120s

lab-down:
	kind delete cluster --name $(CLUSTER)

lab-reset: lab-down lab