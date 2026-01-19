package game

import (
	"sync"
	"time"
)

// Game represents a tic-tac-toe game instance
type Game struct {
	mu sync.RWMutex

	ID        string
	PlayerX   string // First player (creator)
	PlayerO   string // Second player (joiner)
	Board     *Board
	Turn      Mark
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewGame creates a new game with the specified configuration
func NewGame(id, creatorID string, boardSize, winLength int) (*Game, error) {
	board, err := NewBoard(boardSize, winLength)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Game{
		ID:        id,
		PlayerX:   creatorID,
		Board:     board,
		Turn:      MarkX, // X always goes first
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Join adds a second player to the game
func (g *Game) Join(playerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Status != StatusPending {
		return ErrGameAlreadyStarted
	}
	if g.PlayerX == playerID {
		return ErrCannotJoinOwnGame
	}

	g.PlayerO = playerID
	g.Status = StatusInProgress
	g.UpdatedAt = time.Now()
	return nil
}

// MakeMove attempts to place a mark at the given position
func (g *Game) MakeMove(playerID string, row, col int) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Validate game state
	if g.Status != StatusInProgress {
		return ErrGameNotInProgress
	}

	// Validate player
	playerMark := g.getPlayerMark(playerID)
	if playerMark == MarkEmpty {
		return ErrPlayerNotInGame
	}

	// Validate turn
	if g.Turn != playerMark {
		return ErrNotYourTurn
	}

	// Make the move
	if err := g.Board.Set(row, col, playerMark); err != nil {
		return err
	}

	g.UpdatedAt = time.Now()

	// Check for winner
	winner := g.Board.CheckWinner(row, col)
	if winner != MarkEmpty {
		if winner == MarkX {
			g.Status = StatusXWon
		} else {
			g.Status = StatusOWon
		}
		return nil
	}

	// Check for draw
	if g.Board.IsFull() {
		g.Status = StatusDraw
		return nil
	}

	// Switch turn
	g.Turn = g.Turn.Opponent()
	return nil
}

// getPlayerMark returns the mark for the given player ID
func (g *Game) getPlayerMark(playerID string) Mark {
	switch playerID {
	case g.PlayerX:
		return MarkX
	case g.PlayerO:
		return MarkO
	default:
		return MarkEmpty
	}
}

// GetPlayerMark returns the mark for a player (thread-safe)
func (g *Game) GetPlayerMark(playerID string) Mark {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.getPlayerMark(playerID)
}

// GetStatus returns the current game status (thread-safe)
func (g *Game) GetStatus() Status {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Status
}

// GetSnapshot returns a snapshot of the game state
func (g *Game) GetSnapshot() GameSnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return GameSnapshot{
		ID:        g.ID,
		PlayerX:   g.PlayerX,
		PlayerO:   g.PlayerO,
		Board:     g.Board.Clone(),
		Turn:      g.Turn,
		Status:    g.Status,
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
	}
}

// GameSnapshot is an immutable snapshot of game state
type GameSnapshot struct {
	ID        string
	PlayerX   string
	PlayerO   string
	Board     *Board
	Turn      Mark
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GetWinner returns the winner's player ID, or empty string if no winner
func (s *GameSnapshot) GetWinner() string {
	switch s.Status {
	case StatusXWon:
		return s.PlayerX
	case StatusOWon:
		return s.PlayerO
	default:
		return ""
	}
}

// GetLoser returns the loser's player ID, or empty string if no loser
func (s *GameSnapshot) GetLoser() string {
	switch s.Status {
	case StatusXWon:
		return s.PlayerO
	case StatusOWon:
		return s.PlayerX
	default:
		return ""
	}
}

// IsDraw returns true if the game ended in a draw
func (s *GameSnapshot) IsDraw() bool {
	return s.Status == StatusDraw
}
