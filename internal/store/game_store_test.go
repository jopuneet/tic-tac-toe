package store

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tictactoe/internal/game"
)

func TestGameStore_CreateGet(t *testing.T) {
	store := NewGameStore(4)

	g, err := game.NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)

	// Create
	err = store.Create(g)
	require.NoError(t, err)

	// Get
	retrieved, err := store.Get("game-1")
	require.NoError(t, err)
	assert.Equal(t, g.ID, retrieved.ID)

	// Create duplicate
	err = store.Create(g)
	assert.ErrorIs(t, err, ErrGameAlreadyExists)
}

func TestGameStore_GetNotFound(t *testing.T) {
	store := NewGameStore(4)

	_, err := store.Get("nonexistent")
	assert.ErrorIs(t, err, ErrGameNotFound)
}

func TestGameStore_Delete(t *testing.T) {
	store := NewGameStore(4)

	g, _ := game.NewGame("game-1", "player-1", 3, 3)
	store.Create(g)

	err := store.Delete("game-1")
	require.NoError(t, err)

	_, err = store.Get("game-1")
	assert.ErrorIs(t, err, ErrGameNotFound)

	// Delete again
	err = store.Delete("game-1")
	assert.ErrorIs(t, err, ErrGameNotFound)
}

func TestGameStore_ListPending(t *testing.T) {
	store := NewGameStore(4)

	// Create some games
	for i := 0; i < 5; i++ {
		g, _ := game.NewGame(string(rune('a'+i)), "player", 3, 3)
		store.Create(g)
	}

	// Start one game
	g, _ := store.Get("c")
	g.Join("player-2")

	// List pending
	pending, total := store.ListPending(10, 0)
	assert.Equal(t, 4, total) // One game is in progress
	assert.Len(t, pending, 4)

	// Test pagination
	pending, total = store.ListPending(2, 0)
	assert.Equal(t, 4, total)
	assert.Len(t, pending, 2)

	pending, total = store.ListPending(2, 3)
	assert.Equal(t, 4, total)
	assert.Len(t, pending, 1)
}

func TestGameStore_Count(t *testing.T) {
	store := NewGameStore(4)

	assert.Equal(t, 0, store.Count())

	g1, _ := game.NewGame("game-1", "player-1", 3, 3)
	g2, _ := game.NewGame("game-2", "player-2", 3, 3)
	store.Create(g1)
	store.Create(g2)

	assert.Equal(t, 2, store.Count())
}

func TestGameStore_Concurrent(t *testing.T) {
	store := NewGameStore(4)
	var wg sync.WaitGroup

	// Concurrent creates
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			g, _ := game.NewGame(string(rune(id)), "player", 3, 3)
			store.Create(g)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 100, store.Count())

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			store.Get(string(rune(id)))
		}(i)
	}
	wg.Wait()
}
