.PHONY: dev build web clean

# Config 
BINARY  := forge
OUT_DIR := ./bin
WEB_DIR := ./web

# Default: dev mode (run Go + Vite in parallel) 
dev:
	@echo "▶ Starting Forge dev server…"
	@mkdir -p data
	go run ./cmd/forge &
	cd $(WEB_DIR) && pnpm dev

# Build everything 
build: web
	@mkdir -p $(OUT_DIR)
	go build -ldflags="-s -w" -o $(OUT_DIR)/$(BINARY) ./cmd/forge
	@echo "✔ Binary → $(OUT_DIR)/$(BINARY)"

# Build the React frontend 
web:
	cd $(WEB_DIR) && pnpm install && pnpm build
	@echo "✔ Frontend → $(WEB_DIR)/dist"

# Run tests 
test:
	go test ./... -v -count=1

# Lint
lint:
	golangci-lint run ./...

# Swagger
swagger:
	swag init -g cmd/forge/main.go --parseDependency --parseInternal

#  Clean 
clean:
	rm -rf $(OUT_DIR) $(WEB_DIR)/dist data/workspaces
	@echo "✔ Cleaned"
