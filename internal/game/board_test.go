package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBoard(t *testing.T) {
	tests := []struct {
		name      string
		size      int
		winLength int
		wantErr   error
	}{
		{
			name:      "valid 3x3 board",
			size:      3,
			winLength: 3,
			wantErr:   nil,
		},
		{
			name:      "valid 5x5 board with win length 4",
			size:      5,
			winLength: 4,
			wantErr:   nil,
		},
		{
			name:      "invalid board size too small",
			size:      2,
			winLength: 2,
			wantErr:   ErrInvalidBoardSize,
		},
		{
			name:      "invalid win length too small",
			size:      3,
			winLength: 2,
			wantErr:   ErrInvalidWinLength,
		},
		{
			name:      "invalid win length larger than board",
			size:      3,
			winLength: 4,
			wantErr:   ErrInvalidWinLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board, err := NewBoard(tt.size, tt.winLength)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, board)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, board)
				assert.Equal(t, tt.size, board.Size)
				assert.Equal(t, tt.winLength, board.WinLength)
				assert.Len(t, board.Cells, tt.size*tt.size)
			}
		})
	}
}

func TestBoard_GetSet(t *testing.T) {
	board, err := NewBoard(3, 3)
	require.NoError(t, err)

	// Test setting and getting
	err = board.Set(0, 0, MarkX)
	require.NoError(t, err)

	mark, err := board.Get(0, 0)
	require.NoError(t, err)
	assert.Equal(t, MarkX, mark)

	// Test getting empty cell
	mark, err = board.Get(1, 1)
	require.NoError(t, err)
	assert.Equal(t, MarkEmpty, mark)

	// Test invalid position
	_, err = board.Get(-1, 0)
	assert.ErrorIs(t, err, ErrInvalidPosition)

	_, err = board.Get(3, 0)
	assert.ErrorIs(t, err, ErrInvalidPosition)

	// Test setting occupied cell
	err = board.Set(0, 0, MarkO)
	assert.ErrorIs(t, err, ErrCellOccupied)

	// Test setting invalid position
	err = board.Set(5, 5, MarkX)
	assert.ErrorIs(t, err, ErrInvalidPosition)
}

func TestBoard_IsFull(t *testing.T) {
	board, err := NewBoard(3, 3)
	require.NoError(t, err)

	assert.False(t, board.IsFull())

	// Fill the board
	mark := MarkX
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			err := board.Set(row, col, mark)
			require.NoError(t, err)
			mark = mark.Opponent()
		}
	}

	assert.True(t, board.IsFull())
}

func TestBoard_CheckWinner_Horizontal(t *testing.T) {
	board, err := NewBoard(3, 3)
	require.NoError(t, err)

	// X X X
	// O O .
	// . . .
	board.Set(0, 0, MarkX)
	board.Set(1, 0, MarkO)
	board.Set(0, 1, MarkX)
	board.Set(1, 1, MarkO)
	board.Set(0, 2, MarkX)

	winner := board.CheckWinner(0, 2)
	assert.Equal(t, MarkX, winner)
}

func TestBoard_CheckWinner_Vertical(t *testing.T) {
	board, err := NewBoard(3, 3)
	require.NoError(t, err)

	// X O .
	// X O .
	// X . .
	board.Set(0, 0, MarkX)
	board.Set(0, 1, MarkO)
	board.Set(1, 0, MarkX)
	board.Set(1, 1, MarkO)
	board.Set(2, 0, MarkX)

	winner := board.CheckWinner(2, 0)
	assert.Equal(t, MarkX, winner)
}

func TestBoard_CheckWinner_Diagonal(t *testing.T) {
	board, err := NewBoard(3, 3)
	require.NoError(t, err)

	// X O .
	// O X .
	// . . X
	board.Set(0, 0, MarkX)
	board.Set(0, 1, MarkO)
	board.Set(1, 1, MarkX)
	board.Set(1, 0, MarkO)
	board.Set(2, 2, MarkX)

	winner := board.CheckWinner(2, 2)
	assert.Equal(t, MarkX, winner)
}

func TestBoard_CheckWinner_AntiDiagonal(t *testing.T) {
	board, err := NewBoard(3, 3)
	require.NoError(t, err)

	// . O X
	// O X .
	// X . .
	board.Set(0, 2, MarkX)
	board.Set(0, 1, MarkO)
	board.Set(1, 1, MarkX)
	board.Set(1, 0, MarkO)
	board.Set(2, 0, MarkX)

	winner := board.CheckWinner(2, 0)
	assert.Equal(t, MarkX, winner)
}

func TestBoard_CheckWinner_NoWinner(t *testing.T) {
	board, err := NewBoard(3, 3)
	require.NoError(t, err)

	// X O .
	// . X .
	// . . .
	board.Set(0, 0, MarkX)
	board.Set(0, 1, MarkO)
	board.Set(1, 1, MarkX)

	winner := board.CheckWinner(1, 1)
	assert.Equal(t, MarkEmpty, winner)
}

func TestBoard_CheckWinner_LargerBoard(t *testing.T) {
	board, err := NewBoard(5, 4)
	require.NoError(t, err)

	// X X X X .
	// O O O . .
	// . . . . .
	// . . . . .
	// . . . . .
	board.Set(0, 0, MarkX)
	board.Set(1, 0, MarkO)
	board.Set(0, 1, MarkX)
	board.Set(1, 1, MarkO)
	board.Set(0, 2, MarkX)
	board.Set(1, 2, MarkO)
	board.Set(0, 3, MarkX)

	winner := board.CheckWinner(0, 3)
	assert.Equal(t, MarkX, winner)
}

func TestBoard_Clone(t *testing.T) {
	board, err := NewBoard(3, 3)
	require.NoError(t, err)

	board.Set(0, 0, MarkX)
	board.Set(1, 1, MarkO)

	clone := board.Clone()

	// Verify clone has same values
	assert.Equal(t, board.Size, clone.Size)
	assert.Equal(t, board.WinLength, clone.WinLength)
	assert.Equal(t, board.Cells, clone.Cells)

	// Modify original, clone should be unaffected
	board.Set(2, 2, MarkX)

	origMark, _ := board.Get(2, 2)
	cloneMark, _ := clone.Get(2, 2)
	assert.Equal(t, MarkX, origMark)
	assert.Equal(t, MarkEmpty, cloneMark)
}

func TestMark_Opponent(t *testing.T) {
	assert.Equal(t, MarkO, MarkX.Opponent())
	assert.Equal(t, MarkX, MarkO.Opponent())
	assert.Equal(t, MarkEmpty, MarkEmpty.Opponent())
}

func TestMark_String(t *testing.T) {
	assert.Equal(t, "X", MarkX.String())
	assert.Equal(t, "O", MarkO.String())
	assert.Equal(t, " ", MarkEmpty.String())
}

func TestStatus_IsFinished(t *testing.T) {
	assert.False(t, StatusPending.IsFinished())
	assert.False(t, StatusInProgress.IsFinished())
	assert.True(t, StatusXWon.IsFinished())
	assert.True(t, StatusOWon.IsFinished())
	assert.True(t, StatusDraw.IsFinished())
}
