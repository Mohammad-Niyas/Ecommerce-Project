.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: run
run:
	@go run main.go
