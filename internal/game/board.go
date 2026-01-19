package game

import (
	"errors"
	"fmt"
)

// Mark represents a cell state on the board
type Mark int

const (
	MarkEmpty Mark = iota
	MarkX
	MarkO
)

func (m Mark) String() string {
	switch m {
	case MarkEmpty:
		return " "
	case MarkX:
		return "X"
	case MarkO:
		return "O"
	default:
		return "?"
	}
}

// Opponent returns the opposing mark
func (m Mark) Opponent() Mark {
	switch m {
	case MarkX:
		return MarkO
	case MarkO:
		return MarkX
	default:
		return MarkEmpty
	}
}

// Status represents the current status of a game
type Status int

const (
	StatusPending Status = iota
	StatusInProgress
	StatusXWon
	StatusOWon
	StatusDraw
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "PENDING"
	case StatusInProgress:
		return "IN_PROGRESS"
	case StatusXWon:
		return "X_WON"
	case StatusOWon:
		return "O_WON"
	case StatusDraw:
		return "DRAW"
	default:
		return "UNKNOWN"
	}
}

// IsFinished returns true if the game has ended
func (s Status) IsFinished() bool {
	return s == StatusXWon || s == StatusOWon || s == StatusDraw
}

// Common errors
var (
	ErrInvalidBoardSize   = errors.New("invalid board size: must be at least 3")
	ErrInvalidWinLength   = errors.New("invalid win length: must be at least 3 and at most board size")
	ErrInvalidPosition    = errors.New("invalid position: out of bounds")
	ErrCellOccupied       = errors.New("cell is already occupied")
	ErrGameNotInProgress  = errors.New("game is not in progress")
	ErrNotYourTurn        = errors.New("not your turn")
	ErrPlayerNotInGame    = errors.New("player is not part of this game")
	ErrGameAlreadyStarted = errors.New("game has already started")
	ErrCannotJoinOwnGame  = errors.New("cannot join your own game")
)

// Board represents the game board
type Board struct {
	Size      int
	WinLength int
	Cells     []Mark
}

// NewBoard creates a new board with the given size and win length
func NewBoard(size, winLength int) (*Board, error) {
	if size < 3 {
		return nil, ErrInvalidBoardSize
	}
	if winLength < 3 || winLength > size {
		return nil, ErrInvalidWinLength
	}

	cells := make([]Mark, size*size)
	for i := range cells {
		cells[i] = MarkEmpty
	}

	return &Board{
		Size:      size,
		WinLength: winLength,
		Cells:     cells,
	}, nil
}

// Get returns the mark at the given position
func (b *Board) Get(row, col int) (Mark, error) {
	if !b.isValidPosition(row, col) {
		return MarkEmpty, ErrInvalidPosition
	}
	return b.Cells[row*b.Size+col], nil
}

// Set places a mark at the given position
func (b *Board) Set(row, col int, mark Mark) error {
	if !b.isValidPosition(row, col) {
		return ErrInvalidPosition
	}
	idx := row*b.Size + col
	if b.Cells[idx] != MarkEmpty {
		return ErrCellOccupied
	}
	b.Cells[idx] = mark
	return nil
}

// isValidPosition checks if the position is within bounds
func (b *Board) isValidPosition(row, col int) bool {
	return row >= 0 && row < b.Size && col >= 0 && col < b.Size
}

// IsFull returns true if all cells are occupied
func (b *Board) IsFull() bool {
	for _, cell := range b.Cells {
		if cell == MarkEmpty {
			return false
		}
	}
	return true
}

// CheckWinner checks if there's a winner after a move at (row, col)
// Returns the winning mark or MarkEmpty if no winner
func (b *Board) CheckWinner(row, col int) Mark {
	mark, err := b.Get(row, col)
	if err != nil || mark == MarkEmpty {
		return MarkEmpty
	}

	// Check all directions: horizontal, vertical, diagonal, anti-diagonal
	directions := [][2]int{
		{0, 1},  // horizontal
		{1, 0},  // vertical
		{1, 1},  // diagonal
		{1, -1}, // anti-diagonal
	}

	for _, dir := range directions {
		count := 1 // Count the current cell

		// Count in positive direction
		count += b.countInDirection(row, col, dir[0], dir[1], mark)

		// Count in negative direction
		count += b.countInDirection(row, col, -dir[0], -dir[1], mark)

		if count >= b.WinLength {
			return mark
		}
	}

	return MarkEmpty
}

// countInDirection counts consecutive marks in a direction
func (b *Board) countInDirection(row, col, dRow, dCol int, mark Mark) int {
	count := 0
	r, c := row+dRow, col+dCol

	for b.isValidPosition(r, c) {
		if m, _ := b.Get(r, c); m == mark {
			count++
			r += dRow
			c += dCol
		} else {
			break
		}
	}

	return count
}

// Clone creates a deep copy of the board
func (b *Board) Clone() *Board {
	cells := make([]Mark, len(b.Cells))
	copy(cells, b.Cells)
	return &Board{
		Size:      b.Size,
		WinLength: b.WinLength,
		Cells:     cells,
	}
}

// String returns a string representation of the board
func (b *Board) String() string {
	var result string
	for row := 0; row < b.Size; row++ {
		for col := 0; col < b.Size; col++ {
			mark, _ := b.Get(row, col)
			result += fmt.Sprintf("[%s]", mark)
		}
		result += "\n"
	}
	return result
}
