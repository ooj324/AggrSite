.PHONY: all build-frontend build-backend build clean run

# Default target
all: build

# Build the React frontend
build-frontend:
	@echo "==> Building frontend..."
	cd web && npm install && npm run build

# Build the Go backend
build-backend:
	@echo "==> Building Go binary..."
	go build -o aggrsite .

# Build everything
build: build-frontend build-backend
	@echo "==> Build complete!"

# Clean build artifacts
clean:
	@echo "==> Cleaning..."
	rm -f aggrsite
	rm -rf web/dist

# Run the server locally
run: build
	@echo "==> Starting server..."
	./aggrsite
