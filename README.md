# TicTacToe gRPC + REST Server

A production-grade, scalable tic-tac-toe game server implemented in Go with both gRPC and REST APIs.

## Features

- **Dual API Support**: Both gRPC and REST/JSON APIs
- **Swagger UI**: Interactive API documentation and testing in browser
- **OpenAPI Spec**: Auto-generated OpenAPI/Swagger documentation
- **Configurable board size** (NxN) and win length
- **Real-time game updates** via server streaming (gRPC) or Server-Sent Events
- **Thread-safe in-memory storage** with sharding for scalability
- **User statistics** (wins, losses, draws)
- **Comprehensive test suite** (unit + acceptance tests)
- **CORS enabled** for browser access

## Requirements

- Go 1.22+
- Protocol Buffers compiler (`protoc`)
- Make

### Installing Dependencies

```bash
# macOS
brew install protobuf

# Ubuntu/Debian
apt-get install -y protobuf-compiler

# Install Go protoc plugins (run once)
make proto-tools
```

## Quick Start

```bash
# Download dependencies and generate proto files
make deps
make proto

# Build and run the server
make run

# Or run with custom ports
./bin/tictactoe-server -grpc-port 50051 -http-port 8080
```

After starting the server:
- **Swagger UI**: http://localhost:8080/swagger/
- **REST API**: http://localhost:8080/api/v1/...
- **gRPC**: localhost:50051

## REST API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/games` | Create a new game |
| `GET` | `/api/v1/games:pending` | List games waiting for opponents |
| `POST` | `/api/v1/games/{game_id}/join` | Join an existing game |
| `POST` | `/api/v1/games/{game_id}/move` | Make a move |
| `GET` | `/api/v1/games/{game_id}` | Get game state |
| `GET` | `/api/v1/games/{game_id}/board` | Get board as human-readable matrix |
| `GET` | `/api/v1/users/{user_id}/stats` | Get user statistics |
| `GET` | `/api/v1/games/{game_id}/stream` | Stream game updates (SSE) |

## Example Usage

### Using curl (REST API)

```bash
# Create a game
curl -X POST http://localhost:8080/api/v1/games \
  -H "Content-Type: application/json" \
  -d '{"user_id": "alice", "board_size": 3, "win_length": 3}'

# List pending games
curl http://localhost:8080/api/v1/games:pending

# Join a game
curl -X POST http://localhost:8080/api/v1/games/{GAME_ID}/join \
  -H "Content-Type: application/json" \
  -d '{"user_id": "bob"}'

# Make a move (row=0, col=0)
curl -X POST http://localhost:8080/api/v1/games/{GAME_ID}/move \
  -H "Content-Type: application/json" \
  -d '{"user_id": "alice", "row": 0, "col": 0}'

# Get game state
curl http://localhost:8080/api/v1/games/{GAME_ID}

# Get board as matrix (human-readable)
curl http://localhost:8080/api/v1/games/{GAME_ID}/board

# Display just the board (using jq)
curl -s http://localhost:8080/api/v1/games/{GAME_ID}/board | jq -r '.boardDisplay'

# Get user stats
curl http://localhost:8080/api/v1/users/alice/stats
```

### Using grpcurl (gRPC API)

```bash
# Install grpcurl
brew install grpcurl

# Create a game
grpcurl -plaintext -d '{"user_id": "alice", "board_size": 3, "win_length": 3}' \
  localhost:50051 tictactoe.TicTacToeService/CreateGame

# List pending games
grpcurl -plaintext localhost:50051 tictactoe.TicTacToeService/ListPendingGames

# Join a game
grpcurl -plaintext -d '{"user_id": "bob", "game_id": "<GAME_ID>"}' \
  localhost:50051 tictactoe.TicTacToeService/JoinGame

# Make a move
grpcurl -plaintext -d '{"user_id": "alice", "game_id": "<GAME_ID>", "row": 0, "col": 0}' \
  localhost:50051 tictactoe.TicTacToeService/MakeMove

# Stream game updates
grpcurl -plaintext -d '{"game_id": "<GAME_ID>", "user_id": "alice"}' \
  localhost:50051 tictactoe.TicTacToeService/StreamGameUpdates
```

### Using Browser (Swagger UI)

1. Start the server: `make run`
2. Open http://localhost:8080/swagger/ in your browser
3. Use the interactive UI to test all endpoints

## Running Tests

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run acceptance tests only
make test-acceptance

# Run load tests (100+ concurrent users and games)
make test-load

# Run with coverage report
make test-coverage
```

### Load Test Results

The load tests simulate **1000 games running truly in parallel** with 2000 concurrent users:

```
================================================================================
                         LOAD TEST RESULTS
================================================================================
Configuration:
  - Concurrent Users:     2000
  - Total Games:          1000 (all running in parallel)

Results:
  - Completed Games:      1000
  - X Wins:               608 (60.8%)
  - O Wins:               267 (26.7%)
  - Draws:                125 (12.5%)
  - Errors:               0

Performance:
  - Total Duration:       682ms
  - Avg Game Duration:    682µs
  - Games/Second:         1465+
  - Moves/Second:         11220+
  - Total Moves:          7655
================================================================================

=== Sample User Statistics ===
  user-0: W=0 L=0 D=1 Total=1
  user-100: W=1 L=0 D=0 Total=1
  user-500: W=1 L=0 D=0 Total=1
  user-998: W=1 L=0 D=0 Total=1
  user-1999: W=0 L=1 D=0 Total=1
  ...
```

The tests include:
- **1000 Parallel Games**: 1000 games with 2000 users all running simultaneously
- **High Concurrency**: 200 games with 100 concurrent workers
- **Mixed Board Sizes**: Tests with 3x3, 4x4, 5x5, 6x6, and 7x7 boards

## Project Structure

```
.
├── api/
│   ├── proto/                  # Protocol buffer definitions
│   │   └── tictactoe.proto
│   ├── gen/tictactoe/          # Generated Go code
│   │   ├── tictactoe.pb.go
│   │   ├── tictactoe_grpc.pb.go
│   │   └── tictactoe.pb.gw.go  # REST gateway
│   └── swagger/
│       └── tictactoe.swagger.json
├── cmd/
│   └── server/                 # Server entry point
│       └── main.go
├── internal/
│   ├── game/                   # Game logic (board, rules)
│   ├── server/                 # gRPC server implementation
│   ├── store/                  # In-memory data stores
│   └── swagger/                # Swagger UI embed
├── tests/
│   └── acceptance/             # Integration tests
├── third_party/                # Third-party proto files
│   └── google/api/
├── Makefile
├── go.mod
└── README.md
```

## Design Decisions & Tradeoffs

### 1. Dual API Support (gRPC + REST)

**Decision**: Use gRPC-Gateway to provide both gRPC and REST APIs from a single proto definition.

**Rationale**: 
- gRPC for high-performance server-to-server communication
- REST for browser/mobile clients and easy testing
- Single source of truth (proto file) for both APIs

**Tradeoff**: Additional build complexity with proto plugins.

### 2. Sharded In-Memory Storage

**Decision**: Use sharded maps with fine-grained locking instead of a single global lock.

**Rationale**: For millions of concurrent users, a single lock would become a bottleneck. Sharding distributes load across multiple locks, allowing parallel access to different games.

**Tradeoff**: Slightly more complex implementation, and operations that need to scan all games (like `ListPendingGames`) must iterate through all shards.

### 3. Configurable Board Size

**Decision**: Support arbitrary NxN boards with configurable win length.

**Rationale**: Provides flexibility for different game variants (e.g., 5x5 board with 4-in-a-row to win).

**Tradeoff**: The winner detection algorithm is O(1) per move (checks only from the last move position), but larger boards use more memory.

### 4. Embedded Swagger UI

**Decision**: Embed Swagger UI in the binary using Go's embed feature.

**Rationale**: Single binary deployment, no external dependencies for API documentation.

**Tradeoff**: Slightly larger binary size.

### 5. No Database

**Decision**: Keep everything in memory as specified.

**Rationale**: Simplifies deployment and reduces latency.

**Tradeoff**: Data is lost on restart. For production, would add persistence layer (Redis, PostgreSQL) behind the same interface.

## Scalability Considerations

### Current Design (Single Instance)

- Handles thousands of concurrent games
- Sharding reduces lock contention
- Memory-efficient game representation

### For Millions of Users (Future Improvements)

1. **Horizontal Scaling**: Deploy multiple server instances behind a load balancer
2. **Distributed State**: Replace in-memory stores with Redis Cluster
3. **Game Affinity**: Route requests for the same game to the same server
4. **Event Sourcing**: Store game events for replay and analytics
5. **Rate Limiting**: Add per-user rate limits to prevent abuse

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-grpc-port` | 50051 | gRPC server port |
| `-http-port` | 8080 | HTTP/REST server port |
| `-shards` | 64 | Number of shards for data stores |

## License

MIT
