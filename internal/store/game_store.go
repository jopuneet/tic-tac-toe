package store

import (
	"errors"
	"sync"

	"tictactoe/internal/game"
)

var (
	ErrGameNotFound      = errors.New("game not found")
	ErrGameAlreadyExists = errors.New("game already exists")
)

// GameStore provides thread-safe storage for games
// Uses sharding to reduce lock contention for scalability
type GameStore struct {
	shards    []*gameShard
	numShards int
}

type gameShard struct {
	mu    sync.RWMutex
	games map[string]*game.Game
}

// NewGameStore creates a new game store with the specified number of shards
// More shards = less contention but more memory overhead
func NewGameStore(numShards int) *GameStore {
	if numShards < 1 {
		numShards = 64 // Default for good concurrency
	}

	shards := make([]*gameShard, numShards)
	for i := range shards {
		shards[i] = &gameShard{
			games: make(map[string]*game.Game),
		}
	}

	return &GameStore{
		shards:    shards,
		numShards: numShards,
	}
}

// getShard returns the shard for a given game ID
func (s *GameStore) getShard(gameID string) *gameShard {
	// Simple hash function for sharding
	hash := uint32(0)
	for _, c := range gameID {
		hash = hash*31 + uint32(c)
	}
	return s.shards[hash%uint32(s.numShards)]
}

// Create stores a new game
func (s *GameStore) Create(g *game.Game) error {
	shard := s.getShard(g.ID)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, exists := shard.games[g.ID]; exists {
		return ErrGameAlreadyExists
	}

	shard.games[g.ID] = g
	return nil
}

// Get retrieves a game by ID
func (s *GameStore) Get(gameID string) (*game.Game, error) {
	shard := s.getShard(gameID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	g, exists := shard.games[gameID]
	if !exists {
		return nil, ErrGameNotFound
	}

	return g, nil
}

// Delete removes a game by ID
func (s *GameStore) Delete(gameID string) error {
	shard := s.getShard(gameID)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, exists := shard.games[gameID]; !exists {
		return ErrGameNotFound
	}

	delete(shard.games, gameID)
	return nil
}

// ListPending returns all pending games with pagination
func (s *GameStore) ListPending(limit, offset int) ([]*game.GameSnapshot, int) {
	var pending []*game.GameSnapshot

	// Collect pending games from all shards
	for _, shard := range s.shards {
		shard.mu.RLock()
		for _, g := range shard.games {
			if g.GetStatus() == game.StatusPending {
				snapshot := g.GetSnapshot()
				pending = append(pending, &snapshot)
			}
		}
		shard.mu.RUnlock()
	}

	totalCount := len(pending)

	// Apply pagination
	if offset >= len(pending) {
		return []*game.GameSnapshot{}, totalCount
	}

	pending = pending[offset:]
	if limit > 0 && len(pending) > limit {
		pending = pending[:limit]
	}

	return pending, totalCount
}

// Count returns the total number of games
func (s *GameStore) Count() int {
	count := 0
	for _, shard := range s.shards {
		shard.mu.RLock()
		count += len(shard.games)
		shard.mu.RUnlock()
	}
	return count
}
