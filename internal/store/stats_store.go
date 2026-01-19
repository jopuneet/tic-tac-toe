package store

import (
	"sync"
	"sync/atomic"
)

// UserStats holds win/loss/draw statistics for a user
type UserStats struct {
	UserID string
	Wins   int32
	Losses int32
	Draws  int32
}

// TotalGames returns the total number of games played
func (s *UserStats) TotalGames() int32 {
	return s.Wins + s.Losses + s.Draws
}

// StatsStore provides thread-safe storage for user statistics
// Uses sharding similar to GameStore for scalability
type StatsStore struct {
	shards    []*statsShard
	numShards int
}

type statsShard struct {
	mu    sync.RWMutex
	stats map[string]*UserStats
}

// NewStatsStore creates a new stats store with the specified number of shards
func NewStatsStore(numShards int) *StatsStore {
	if numShards < 1 {
		numShards = 64
	}

	shards := make([]*statsShard, numShards)
	for i := range shards {
		shards[i] = &statsShard{
			stats: make(map[string]*UserStats),
		}
	}

	return &StatsStore{
		shards:    shards,
		numShards: numShards,
	}
}

// getShard returns the shard for a given user ID
func (s *StatsStore) getShard(userID string) *statsShard {
	hash := uint32(0)
	for _, c := range userID {
		hash = hash*31 + uint32(c)
	}
	return s.shards[hash%uint32(s.numShards)]
}

// getOrCreate returns existing stats or creates new ones
func (s *StatsStore) getOrCreate(userID string) *UserStats {
	shard := s.getShard(userID)

	// Try read lock first
	shard.mu.RLock()
	stats, exists := shard.stats[userID]
	shard.mu.RUnlock()

	if exists {
		return stats
	}

	// Need to create - use write lock
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Double-check after acquiring write lock
	if stats, exists = shard.stats[userID]; exists {
		return stats
	}

	stats = &UserStats{UserID: userID}
	shard.stats[userID] = stats
	return stats
}

// Get returns stats for a user
func (s *StatsStore) Get(userID string) UserStats {
	stats := s.getOrCreate(userID)
	return UserStats{
		UserID: userID,
		Wins:   atomic.LoadInt32(&stats.Wins),
		Losses: atomic.LoadInt32(&stats.Losses),
		Draws:  atomic.LoadInt32(&stats.Draws),
	}
}

// RecordWin records a win for a user
func (s *StatsStore) RecordWin(userID string) {
	stats := s.getOrCreate(userID)
	atomic.AddInt32(&stats.Wins, 1)
}

// RecordLoss records a loss for a user
func (s *StatsStore) RecordLoss(userID string) {
	stats := s.getOrCreate(userID)
	atomic.AddInt32(&stats.Losses, 1)
}

// RecordDraw records a draw for a user
func (s *StatsStore) RecordDraw(userID string) {
	stats := s.getOrCreate(userID)
	atomic.AddInt32(&stats.Draws, 1)
}

// RecordGameResult records the result for both players
func (s *StatsStore) RecordGameResult(winnerID, loserID string, isDraw bool) {
	if isDraw {
		if winnerID != "" {
			s.RecordDraw(winnerID)
		}
		if loserID != "" {
			s.RecordDraw(loserID)
		}
	} else {
		if winnerID != "" {
			s.RecordWin(winnerID)
		}
		if loserID != "" {
			s.RecordLoss(loserID)
		}
	}
}
