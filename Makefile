.PHONY: install dev start stop backend agent frontend build clean

# Install all dependencies
install:
	@echo "Installing Go backend dependencies..."
	cd backend && go mod download
	@echo "Installing Python agent dependencies..."
	cd agent && (test -d venv || python3 -m venv venv) && . venv/bin/activate && pip install -r requirements.txt
	@echo "Installing frontend dependencies..."
	cd frontend && npm install
	@echo "Done!"

# ============================================================================
# Development Commands
# ============================================================================

# Development instructions
dev:
	@echo "Starting development..."
	@echo "Run these commands in separate terminals:"
	@echo ""
	@echo "  Terminal 1: make backend   # Go backend on :8080"
	@echo "  Terminal 2: make agent     # Python agent on :8082"
	@echo "  Terminal 3: make frontend  # React frontend on :5173"
	@echo ""
	@echo "Or use 'make start' to run backend and agent in background."

# Start backend and agent in background (frontend needs separate terminal)
start:
	@echo "Starting backend and agent in background..."
	@cd backend && go run cmd/server/main.go &
	@sleep 2
	@cd agent && . venv/bin/activate && python server.py &
	@sleep 2
	@echo ""
	@echo "Backend running on http://localhost:8080"
	@echo "Agent running on http://localhost:8082"
	@echo ""
	@echo "Now run 'make frontend' in this terminal for the UI"

# Stop background services
stop:
	@echo "Stopping services..."
	-@lsof -ti:8080 | xargs kill -9 2>/dev/null || true
	-@lsof -ti:8082 | xargs kill -9 2>/dev/null || true
	@echo "Services stopped."

# Run Go backend (port 8080)
backend:
	cd backend && go run cmd/server/main.go

# Run Python agent (port 8082)
agent:
	cd agent && . venv/bin/activate && python server.py

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
	@echo "Dynamiq - AI-Powered Data Analyst"
	@echo "=================================="
	@echo ""
	@echo "Quick Start:"
	@echo "  make install   - Install all dependencies (first time setup)"
	@echo "  make dev       - Show development instructions"
	@echo ""
	@echo "Individual Services (run in separate terminals):"
	@echo "  make backend   - Run Go backend on :8080"
	@echo "  make agent     - Run Python agent on :8082"
	@echo "  make frontend  - Run frontend on :5173"
	@echo ""
	@echo "Background Mode:"
	@echo "  make start     - Start backend + agent in background"
	@echo "  make stop      - Stop background services"
	@echo ""
	@echo "Build & Clean:"
	@echo "  make build     - Build Go backend binary"
	@echo "  make clean     - Clean build artifacts"
	@echo ""
	@echo "Database Connections:"
	@echo "  Connect databases (PostgreSQL, MySQL, etc.) via the Integrations panel"
	@echo "  in the UI. Credentials are stored locally and injected into sandboxes."
