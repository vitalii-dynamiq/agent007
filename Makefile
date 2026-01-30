.PHONY: install dev backend agent frontend build clean

# Install all dependencies
install:
	@echo "Installing Go backend dependencies..."
	cd backend && go mod download
	@echo "Installing Python agent dependencies..."
	cd agent && pip install -r requirements.txt
	@echo "Installing frontend dependencies..."
	cd frontend && npm install
	@echo "Done!"

# Run Go backend (port 8080)
backend:
	cd backend && go run cmd/server/main.go

# Run Python agent (port 8081)
agent:
	cd agent && python server.py

# Run frontend (port 5173)
frontend:
	cd frontend && npm run dev

# Build Go backend
build:
	cd backend && go build -o bin/server cmd/server/main.go

# Clean
clean:
	rm -rf backend/bin
	rm -rf frontend/dist
	rm -rf agent/__pycache__

# Help
help:
	@echo "Dynamiq Development Commands:"
	@echo ""
	@echo "  make install   - Install all dependencies"
	@echo "  make backend   - Run Go backend on :8080"
	@echo "  make agent     - Run Python agent on :8081"
	@echo "  make frontend  - Run frontend on :5173"
	@echo "  make build     - Build Go backend"
	@echo "  make clean     - Clean build artifacts"
	@echo ""
	@echo "Run each service in a separate terminal."
