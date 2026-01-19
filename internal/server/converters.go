package server

import (
	pb "tictactoe/api/gen/tictactoe"
	"tictactoe/internal/game"
)

// gameToProto converts a GameSnapshot to protobuf Game message
func gameToProto(snapshot game.GameSnapshot) *pb.Game {
	board := make([]pb.Mark, len(snapshot.Board.Cells))
	for i, cell := range snapshot.Board.Cells {
		board[i] = markToProto(cell)
	}

	return &pb.Game{
		GameId:    snapshot.ID,
		PlayerXId: snapshot.PlayerX,
		PlayerOId: snapshot.PlayerO,
		BoardSize: int32(snapshot.Board.Size),
		WinLength: int32(snapshot.Board.WinLength),
		Board:     board,
		CurrentTurn: markToProto(snapshot.Turn),
		Status:    statusToProto(snapshot.Status),
		CreatedAt: snapshot.CreatedAt.Unix(),
		UpdatedAt: snapshot.UpdatedAt.Unix(),
	}
}

// markToProto converts a game.Mark to protobuf Mark
func markToProto(m game.Mark) pb.Mark {
	switch m {
	case game.MarkEmpty:
		return pb.Mark_MARK_EMPTY
	case game.MarkX:
		return pb.Mark_MARK_X
	case game.MarkO:
		return pb.Mark_MARK_O
	default:
		return pb.Mark_MARK_UNSPECIFIED
	}
}

// statusToProto converts a game.Status to protobuf GameStatus
func statusToProto(s game.Status) pb.GameStatus {
	switch s {
	case game.StatusPending:
		return pb.GameStatus_GAME_STATUS_PENDING
	case game.StatusInProgress:
		return pb.GameStatus_GAME_STATUS_IN_PROGRESS
	case game.StatusXWon:
		return pb.GameStatus_GAME_STATUS_X_WON
	case game.StatusOWon:
		return pb.GameStatus_GAME_STATUS_O_WON
	case game.StatusDraw:
		return pb.GameStatus_GAME_STATUS_DRAW
	default:
		return pb.GameStatus_GAME_STATUS_UNSPECIFIED
	}
}
