localdev-run-authorizer:
	GIN_MODE=debug ENVIRONMENT=dev go run cmd/authorizer/main.go
