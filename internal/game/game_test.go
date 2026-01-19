package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGame(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)

	assert.Equal(t, "game-1", g.ID)
	assert.Equal(t, "player-1", g.PlayerX)
	assert.Empty(t, g.PlayerO)
	assert.Equal(t, MarkX, g.Turn)
	assert.Equal(t, StatusPending, g.Status)
	assert.NotNil(t, g.Board)
}

func TestGame_Join(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)

	// Join the game
	err = g.Join("player-2")
	require.NoError(t, err)

	assert.Equal(t, "player-2", g.PlayerO)
	assert.Equal(t, StatusInProgress, g.Status)

	// Cannot join again
	err = g.Join("player-3")
	assert.ErrorIs(t, err, ErrGameAlreadyStarted)
}

func TestGame_Join_CannotJoinOwnGame(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)

	err = g.Join("player-1")
	assert.ErrorIs(t, err, ErrCannotJoinOwnGame)
}

func TestGame_MakeMove(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)
	g.Join("player-2")

	// Player X makes a move
	err = g.MakeMove("player-1", 0, 0)
	require.NoError(t, err)

	mark, _ := g.Board.Get(0, 0)
	assert.Equal(t, MarkX, mark)
	assert.Equal(t, MarkO, g.Turn)

	// Player O makes a move
	err = g.MakeMove("player-2", 1, 1)
	require.NoError(t, err)

	mark, _ = g.Board.Get(1, 1)
	assert.Equal(t, MarkO, mark)
	assert.Equal(t, MarkX, g.Turn)
}

func TestGame_MakeMove_NotYourTurn(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)
	g.Join("player-2")

	// Player O tries to move first
	err = g.MakeMove("player-2", 0, 0)
	assert.ErrorIs(t, err, ErrNotYourTurn)
}

func TestGame_MakeMove_PlayerNotInGame(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)
	g.Join("player-2")

	err = g.MakeMove("player-3", 0, 0)
	assert.ErrorIs(t, err, ErrPlayerNotInGame)
}

func TestGame_MakeMove_GameNotInProgress(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)

	// Game is still pending
	err = g.MakeMove("player-1", 0, 0)
	assert.ErrorIs(t, err, ErrGameNotInProgress)
}

func TestGame_MakeMove_WinCondition(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)
	g.Join("player-2")

	// X wins with horizontal line
	// X X X
	// O O .
	// . . .
	moves := []struct {
		player string
		row    int
		col    int
	}{
		{"player-1", 0, 0}, // X
		{"player-2", 1, 0}, // O
		{"player-1", 0, 1}, // X
		{"player-2", 1, 1}, // O
		{"player-1", 0, 2}, // X wins
	}

	for _, m := range moves {
		err := g.MakeMove(m.player, m.row, m.col)
		require.NoError(t, err)
	}

	assert.Equal(t, StatusXWon, g.Status)
}

func TestGame_MakeMove_DrawCondition(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)
	g.Join("player-2")

	// Draw scenario
	// X O X
	// X X O
	// O X O
	moves := []struct {
		player string
		row    int
		col    int
	}{
		{"player-1", 0, 0}, // X
		{"player-2", 0, 1}, // O
		{"player-1", 0, 2}, // X
		{"player-2", 1, 2}, // O
		{"player-1", 1, 0}, // X
		{"player-2", 2, 0}, // O
		{"player-1", 1, 1}, // X
		{"player-2", 2, 2}, // O
		{"player-1", 2, 1}, // X - draw
	}

	for _, m := range moves {
		err := g.MakeMove(m.player, m.row, m.col)
		require.NoError(t, err)
	}

	assert.Equal(t, StatusDraw, g.Status)
}

func TestGame_GetSnapshot(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)
	g.Join("player-2")
	g.MakeMove("player-1", 0, 0)

	snapshot := g.GetSnapshot()

	assert.Equal(t, g.ID, snapshot.ID)
	assert.Equal(t, g.PlayerX, snapshot.PlayerX)
	assert.Equal(t, g.PlayerO, snapshot.PlayerO)
	assert.Equal(t, g.Turn, snapshot.Turn)
	assert.Equal(t, g.Status, snapshot.Status)

	// Verify board is a copy
	snapshot.Board.Set(1, 1, MarkO)
	origMark, _ := g.Board.Get(1, 1)
	assert.Equal(t, MarkEmpty, origMark)
}

func TestGameSnapshot_GetWinnerLoser(t *testing.T) {
	g, err := NewGame("game-1", "player-1", 3, 3)
	require.NoError(t, err)
	g.Join("player-2")

	// Make X win
	g.MakeMove("player-1", 0, 0)
	g.MakeMove("player-2", 1, 0)
	g.MakeMove("player-1", 0, 1)
	g.MakeMove("player-2", 1, 1)
	g.MakeMove("player-1", 0, 2)

	snapshot := g.GetSnapshot()

	assert.Equal(t, "player-1", snapshot.GetWinner())
	assert.Equal(t, "player-2", snapshot.GetLoser())
	assert.False(t, snapshot.IsDraw())
}
