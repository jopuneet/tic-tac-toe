package store

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatsStore_Get(t *testing.T) {
	store := NewStatsStore(4)

	// Get non-existent user returns zero stats
	stats := store.Get("user-1")
	assert.Equal(t, "user-1", stats.UserID)
	assert.Equal(t, int32(0), stats.Wins)
	assert.Equal(t, int32(0), stats.Losses)
	assert.Equal(t, int32(0), stats.Draws)
}

func TestStatsStore_RecordWin(t *testing.T) {
	store := NewStatsStore(4)

	store.RecordWin("user-1")
	store.RecordWin("user-1")
	store.RecordWin("user-1")

	stats := store.Get("user-1")
	assert.Equal(t, int32(3), stats.Wins)
	assert.Equal(t, int32(0), stats.Losses)
	assert.Equal(t, int32(0), stats.Draws)
}

func TestStatsStore_RecordLoss(t *testing.T) {
	store := NewStatsStore(4)

	store.RecordLoss("user-1")
	store.RecordLoss("user-1")

	stats := store.Get("user-1")
	assert.Equal(t, int32(0), stats.Wins)
	assert.Equal(t, int32(2), stats.Losses)
	assert.Equal(t, int32(0), stats.Draws)
}

func TestStatsStore_RecordDraw(t *testing.T) {
	store := NewStatsStore(4)

	store.RecordDraw("user-1")

	stats := store.Get("user-1")
	assert.Equal(t, int32(0), stats.Wins)
	assert.Equal(t, int32(0), stats.Losses)
	assert.Equal(t, int32(1), stats.Draws)
}

func TestStatsStore_RecordGameResult(t *testing.T) {
	store := NewStatsStore(4)

	// Record a win/loss
	store.RecordGameResult("winner", "loser", false)

	winnerStats := store.Get("winner")
	assert.Equal(t, int32(1), winnerStats.Wins)
	assert.Equal(t, int32(0), winnerStats.Losses)

	loserStats := store.Get("loser")
	assert.Equal(t, int32(0), loserStats.Wins)
	assert.Equal(t, int32(1), loserStats.Losses)

	// Record a draw
	store.RecordGameResult("player1", "player2", true)

	p1Stats := store.Get("player1")
	assert.Equal(t, int32(1), p1Stats.Draws)

	p2Stats := store.Get("player2")
	assert.Equal(t, int32(1), p2Stats.Draws)
}

func TestStatsStore_TotalGames(t *testing.T) {
	store := NewStatsStore(4)

	store.RecordWin("user-1")
	store.RecordWin("user-1")
	store.RecordLoss("user-1")
	store.RecordDraw("user-1")

	stats := store.Get("user-1")
	assert.Equal(t, int32(4), stats.TotalGames())
}

func TestStatsStore_Concurrent(t *testing.T) {
	store := NewStatsStore(4)
	var wg sync.WaitGroup

	// Concurrent updates to same user
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			store.RecordWin("user-1")
		}()
		go func() {
			defer wg.Done()
			store.RecordLoss("user-1")
		}()
		go func() {
			defer wg.Done()
			store.RecordDraw("user-1")
		}()
	}
	wg.Wait()

	stats := store.Get("user-1")
	assert.Equal(t, int32(100), stats.Wins)
	assert.Equal(t, int32(100), stats.Losses)
	assert.Equal(t, int32(100), stats.Draws)
	assert.Equal(t, int32(300), stats.TotalGames())
}
