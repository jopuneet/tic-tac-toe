package server

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "tictactoe/api/gen/tictactoe"
	"tictactoe/internal/game"
	"tictactoe/internal/store"
)

const (
	DefaultBoardSize  = 3
	DefaultWinLength  = 3
	DefaultListLimit  = 50
	MaxBoardSize      = 20
	MaxListLimit      = 100
)

// TicTacToeServer implements the gRPC TicTacToeService
type TicTacToeServer struct {
	pb.UnimplementedTicTacToeServiceServer

	gameStore  *store.GameStore
	statsStore *store.StatsStore

	// Subscribers for game updates (gameID -> set of channels)
	subscribersMu sync.RWMutex
	subscribers   map[string]map[chan *pb.GameUpdate]struct{}
}

// NewTicTacToeServer creates a new server instance
func NewTicTacToeServer(gameStore *store.GameStore, statsStore *store.StatsStore) *TicTacToeServer {
	return &TicTacToeServer{
		gameStore:   gameStore,
		statsStore:  statsStore,
		subscribers: make(map[string]map[chan *pb.GameUpdate]struct{}),
	}
}

// CreateGame creates a new game and waits for an opponent
func (s *TicTacToeServer) CreateGame(ctx context.Context, req *pb.CreateGameRequest) (*pb.CreateGameResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	boardSize := int(req.BoardSize)
	if boardSize == 0 {
		boardSize = DefaultBoardSize
	}
	if boardSize < 3 || boardSize > MaxBoardSize {
		return nil, status.Errorf(codes.InvalidArgument, "board_size must be between 3 and %d", MaxBoardSize)
	}

	winLength := int(req.WinLength)
	if winLength == 0 {
		winLength = DefaultWinLength
	}
	if winLength < 3 || winLength > boardSize {
		return nil, status.Errorf(codes.InvalidArgument, "win_length must be between 3 and board_size (%d)", boardSize)
	}

	gameID := uuid.New().String()
	g, err := game.NewGame(gameID, req.UserId, boardSize, winLength)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create game: %v", err)
	}

	if err := s.gameStore.Create(g); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to store game: %v", err)
	}

	return &pb.CreateGameResponse{
		Game: gameToProto(g.GetSnapshot()),
	}, nil
}

// ListPendingGames returns all games waiting for an opponent
func (s *TicTacToeServer) ListPendingGames(ctx context.Context, req *pb.ListPendingGamesRequest) (*pb.ListPendingGamesResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}

	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	games, totalCount := s.gameStore.ListPending(limit, offset)

	pbGames := make([]*pb.Game, len(games))
	for i, g := range games {
		pbGames[i] = gameToProto(*g)
	}

	return &pb.ListPendingGamesResponse{
		Games:      pbGames,
		TotalCount: int32(totalCount),
	}, nil
}

// JoinGame joins an existing pending game
func (s *TicTacToeServer) JoinGame(ctx context.Context, req *pb.JoinGameRequest) (*pb.JoinGameResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GameId == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id is required")
	}

	g, err := s.gameStore.Get(req.GameId)
	if err != nil {
		if err == store.ErrGameNotFound {
			return nil, status.Error(codes.NotFound, "game not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get game: %v", err)
	}

	if err := g.Join(req.UserId); err != nil {
		switch err {
		case game.ErrGameAlreadyStarted:
			return nil, status.Error(codes.FailedPrecondition, "game has already started")
		case game.ErrCannotJoinOwnGame:
			return nil, status.Error(codes.InvalidArgument, "cannot join your own game")
		default:
			return nil, status.Errorf(codes.Internal, "failed to join game: %v", err)
		}
	}

	snapshot := g.GetSnapshot()

	// Notify subscribers that the game has started
	s.broadcastUpdate(req.GameId, &pb.GameUpdate{
		Game:    gameToProto(snapshot),
		Message: "Game started! Player X's turn.",
	})

	return &pb.JoinGameResponse{
		Game: gameToProto(snapshot),
	}, nil
}

// MakeMove makes a move in an active game
func (s *TicTacToeServer) MakeMove(ctx context.Context, req *pb.MakeMoveRequest) (*pb.MakeMoveResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GameId == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id is required")
	}

	g, err := s.gameStore.Get(req.GameId)
	if err != nil {
		if err == store.ErrGameNotFound {
			return nil, status.Error(codes.NotFound, "game not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get game: %v", err)
	}

	if err := g.MakeMove(req.UserId, int(req.Row), int(req.Col)); err != nil {
		switch err {
		case game.ErrGameNotInProgress:
			return nil, status.Error(codes.FailedPrecondition, "game is not in progress")
		case game.ErrPlayerNotInGame:
			return nil, status.Error(codes.PermissionDenied, "you are not a player in this game")
		case game.ErrNotYourTurn:
			return nil, status.Error(codes.FailedPrecondition, "it's not your turn")
		case game.ErrInvalidPosition:
			return nil, status.Error(codes.InvalidArgument, "invalid position")
		case game.ErrCellOccupied:
			return nil, status.Error(codes.InvalidArgument, "cell is already occupied")
		default:
			return nil, status.Errorf(codes.Internal, "failed to make move: %v", err)
		}
	}

	snapshot := g.GetSnapshot()

	// Update stats if game is finished
	if snapshot.Status.IsFinished() {
		s.recordGameResult(snapshot)
	}

	// Broadcast update
	s.broadcastUpdate(req.GameId, &pb.GameUpdate{
		Game:    gameToProto(snapshot),
		Message: s.getUpdateMessage(snapshot),
	})

	return &pb.MakeMoveResponse{
		Game: gameToProto(snapshot),
	}, nil
}

// GetGame retrieves the current state of a game
func (s *TicTacToeServer) GetGame(ctx context.Context, req *pb.GetGameRequest) (*pb.GetGameResponse, error) {
	if req.GameId == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id is required")
	}

	g, err := s.gameStore.Get(req.GameId)
	if err != nil {
		if err == store.ErrGameNotFound {
			return nil, status.Error(codes.NotFound, "game not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get game: %v", err)
	}

	return &pb.GetGameResponse{
		Game: gameToProto(g.GetSnapshot()),
	}, nil
}

// GetGameBoard retrieves the game board as a human-readable matrix
func (s *TicTacToeServer) GetGameBoard(ctx context.Context, req *pb.GetGameBoardRequest) (*pb.GetGameBoardResponse, error) {
	if req.GameId == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id is required")
	}

	g, err := s.gameStore.Get(req.GameId)
	if err != nil {
		if err == store.ErrGameNotFound {
			return nil, status.Error(codes.NotFound, "game not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get game: %v", err)
	}

	snapshot := g.GetSnapshot()
	return snapshotToBoardResponse(snapshot), nil
}

// snapshotToBoardResponse converts a game snapshot to a board response
func snapshotToBoardResponse(snapshot game.GameSnapshot) *pb.GetGameBoardResponse {
	size := snapshot.Board.Size
	rows := make([]string, size)
	var displayBuilder strings.Builder

	// Build separator line
	separator := "+" + strings.Repeat("---+", size)

	displayBuilder.WriteString(separator + "\n")

	for row := 0; row < size; row++ {
		var rowCells []string
		for col := 0; col < size; col++ {
			mark, _ := snapshot.Board.Get(row, col)
			rowCells = append(rowCells, markToChar(mark))
		}
		rows[row] = strings.Join(rowCells, "|")

		// Build display string with borders
		displayBuilder.WriteString("| ")
		displayBuilder.WriteString(strings.Join(rowCells, " | "))
		displayBuilder.WriteString(" |\n")
		displayBuilder.WriteString(separator + "\n")
	}

	// Get status string
	statusStr := getStatusString(snapshot.Status)

	// Get current turn
	turnStr := "N/A"
	if snapshot.Status == game.StatusInProgress {
		turnStr = markToChar(snapshot.Turn)
	}

	return &pb.GetGameBoardResponse{
		GameId:       snapshot.ID,
		BoardSize:    int32(size),
		Rows:         rows,
		BoardDisplay: displayBuilder.String(),
		Status:       statusStr,
		CurrentTurn:  turnStr,
		PlayerX:      snapshot.PlayerX,
		PlayerO:      snapshot.PlayerO,
	}
}

// markToChar converts a Mark to a display character
func markToChar(m game.Mark) string {
	switch m {
	case game.MarkX:
		return "X"
	case game.MarkO:
		return "O"
	default:
		return " "
	}
}

// getStatusString returns a human-readable status
func getStatusString(s game.Status) string {
	switch s {
	case game.StatusPending:
		return "Waiting for opponent"
	case game.StatusInProgress:
		return "Game in progress"
	case game.StatusXWon:
		return "Player X won!"
	case game.StatusOWon:
		return "Player O won!"
	case game.StatusDraw:
		return "Game ended in a draw"
	default:
		return "Unknown"
	}
}

// GetUserStats retrieves win-lose-draw statistics for a user
func (s *TicTacToeServer) GetUserStats(ctx context.Context, req *pb.GetUserStatsRequest) (*pb.GetUserStatsResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	stats := s.statsStore.Get(req.UserId)

	return &pb.GetUserStatsResponse{
		UserId:     stats.UserID,
		Wins:       stats.Wins,
		Losses:     stats.Losses,
		Draws:      stats.Draws,
		TotalGames: stats.TotalGames(),
	}, nil
}

// StreamGameUpdates streams game state updates to connected players
func (s *TicTacToeServer) StreamGameUpdates(req *pb.StreamGameUpdatesRequest, stream pb.TicTacToeService_StreamGameUpdatesServer) error {
	if req.GameId == "" {
		return status.Error(codes.InvalidArgument, "game_id is required")
	}

	// Verify game exists
	g, err := s.gameStore.Get(req.GameId)
	if err != nil {
		if err == store.ErrGameNotFound {
			return status.Error(codes.NotFound, "game not found")
		}
		return status.Errorf(codes.Internal, "failed to get game: %v", err)
	}

	// Create channel for updates
	updateCh := make(chan *pb.GameUpdate, 10)
	s.subscribe(req.GameId, updateCh)
	defer s.unsubscribe(req.GameId, updateCh)

	// Send initial state
	if err := stream.Send(&pb.GameUpdate{
		Game:    gameToProto(g.GetSnapshot()),
		Message: "Connected to game",
	}); err != nil {
		return err
	}

	// Stream updates
	for {
		select {
		case update := <-updateCh:
			if err := stream.Send(update); err != nil {
				return err
			}
			// Check if game is finished
			if update.Game != nil && isGameFinished(update.Game.Status) {
				return nil
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// subscribe adds a channel to receive updates for a game
func (s *TicTacToeServer) subscribe(gameID string, ch chan *pb.GameUpdate) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	if s.subscribers[gameID] == nil {
		s.subscribers[gameID] = make(map[chan *pb.GameUpdate]struct{})
	}
	s.subscribers[gameID][ch] = struct{}{}
}

// unsubscribe removes a channel from receiving updates
func (s *TicTacToeServer) unsubscribe(gameID string, ch chan *pb.GameUpdate) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	if subs, ok := s.subscribers[gameID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(s.subscribers, gameID)
		}
	}
	close(ch)
}

// broadcastUpdate sends an update to all subscribers of a game
func (s *TicTacToeServer) broadcastUpdate(gameID string, update *pb.GameUpdate) {
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()

	if subs, ok := s.subscribers[gameID]; ok {
		for ch := range subs {
			select {
			case ch <- update:
			default:
				// Channel full, skip (non-blocking)
			}
		}
	}
}

// recordGameResult records the game result in stats
func (s *TicTacToeServer) recordGameResult(snapshot game.GameSnapshot) {
	if snapshot.IsDraw() {
		s.statsStore.RecordGameResult(snapshot.PlayerX, snapshot.PlayerO, true)
	} else {
		s.statsStore.RecordGameResult(snapshot.GetWinner(), snapshot.GetLoser(), false)
	}
}

// getUpdateMessage generates a human-readable message for a game state
func (s *TicTacToeServer) getUpdateMessage(snapshot game.GameSnapshot) string {
	switch snapshot.Status {
	case game.StatusXWon:
		return "Player X wins!"
	case game.StatusOWon:
		return "Player O wins!"
	case game.StatusDraw:
		return "Game ended in a draw!"
	case game.StatusInProgress:
		if snapshot.Turn == game.MarkX {
			return "Player X's turn"
		}
		return "Player O's turn"
	default:
		return ""
	}
}

// isGameFinished checks if a game status indicates completion
func isGameFinished(status pb.GameStatus) bool {
	return status == pb.GameStatus_GAME_STATUS_X_WON ||
		status == pb.GameStatus_GAME_STATUS_O_WON ||
		status == pb.GameStatus_GAME_STATUS_DRAW
}
