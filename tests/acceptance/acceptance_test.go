package acceptance

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "tictactoe/api/gen/tictactoe"
	"tictactoe/internal/server"
	"tictactoe/internal/store"
)

// testServer holds the server and client for acceptance tests
type testServer struct {
	grpcServer *grpc.Server
	client     pb.TicTacToeServiceClient
	conn       *grpc.ClientConn
	addr       string
}

func setupTestServer(t *testing.T) *testServer {
	// Create stores
	gameStore := store.NewGameStore(4)
	statsStore := store.NewStatsStore(4)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	ticTacToeServer := server.NewTicTacToeServer(gameStore, statsStore)
	pb.RegisterTicTacToeServiceServer(grpcServer, ticTacToeServer)

	// Start listening on random port
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	go grpcServer.Serve(listener)

	// Create client
	addr := listener.Addr().String()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	client := pb.NewTicTacToeServiceClient(conn)

	return &testServer{
		grpcServer: grpcServer,
		client:     client,
		conn:       conn,
		addr:       addr,
	}
}

func (ts *testServer) cleanup() {
	ts.conn.Close()
	ts.grpcServer.Stop()
}

func TestAcceptance_CreateGame(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create a game
	resp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId:    "player-1",
		BoardSize: 3,
		WinLength: 3,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Game.GameId)
	assert.Equal(t, "player-1", resp.Game.PlayerXId)
	assert.Empty(t, resp.Game.PlayerOId)
	assert.Equal(t, int32(3), resp.Game.BoardSize)
	assert.Equal(t, int32(3), resp.Game.WinLength)
	assert.Equal(t, pb.GameStatus_GAME_STATUS_PENDING, resp.Game.Status)
	assert.Equal(t, pb.Mark_MARK_X, resp.Game.CurrentTurn)
}

func TestAcceptance_CreateGame_DefaultValues(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create a game with defaults
	resp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)

	assert.Equal(t, int32(3), resp.Game.BoardSize)
	assert.Equal(t, int32(3), resp.Game.WinLength)
}

func TestAcceptance_CreateGame_CustomSize(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create a 5x5 game with win length 4
	resp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId:    "player-1",
		BoardSize: 5,
		WinLength: 4,
	})
	require.NoError(t, err)

	assert.Equal(t, int32(5), resp.Game.BoardSize)
	assert.Equal(t, int32(4), resp.Game.WinLength)
	assert.Len(t, resp.Game.Board, 25)
}

func TestAcceptance_CreateGame_InvalidInput(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Missing user ID
	_, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	// Invalid board size
	_, err = ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId:    "player-1",
		BoardSize: 2,
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	// Invalid win length
	_, err = ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId:    "player-1",
		BoardSize: 3,
		WinLength: 5,
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAcceptance_ListPendingGames(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create some games
	for i := 0; i < 5; i++ {
		_, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
			UserId: fmt.Sprintf("player-%d", i),
		})
		require.NoError(t, err)
	}

	// List pending games
	resp, err := ts.client.ListPendingGames(ctx, &pb.ListPendingGamesRequest{})
	require.NoError(t, err)

	assert.Equal(t, int32(5), resp.TotalCount)
	assert.Len(t, resp.Games, 5)

	// Test pagination
	resp, err = ts.client.ListPendingGames(ctx, &pb.ListPendingGamesRequest{
		Limit:  2,
		Offset: 0,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Games, 2)
	assert.Equal(t, int32(5), resp.TotalCount)
}

func TestAcceptance_JoinGame(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create a game
	createResp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)

	gameID := createResp.Game.GameId

	// Join the game
	joinResp, err := ts.client.JoinGame(ctx, &pb.JoinGameRequest{
		UserId: "player-2",
		GameId: gameID,
	})
	require.NoError(t, err)

	assert.Equal(t, "player-2", joinResp.Game.PlayerOId)
	assert.Equal(t, pb.GameStatus_GAME_STATUS_IN_PROGRESS, joinResp.Game.Status)

	// Verify game is no longer in pending list
	listResp, err := ts.client.ListPendingGames(ctx, &pb.ListPendingGamesRequest{})
	require.NoError(t, err)
	assert.Equal(t, int32(0), listResp.TotalCount)
}

func TestAcceptance_JoinGame_Errors(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create a game
	createResp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)

	gameID := createResp.Game.GameId

	// Cannot join own game
	_, err = ts.client.JoinGame(ctx, &pb.JoinGameRequest{
		UserId: "player-1",
		GameId: gameID,
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	// Join the game
	_, err = ts.client.JoinGame(ctx, &pb.JoinGameRequest{
		UserId: "player-2",
		GameId: gameID,
	})
	require.NoError(t, err)

	// Cannot join again
	_, err = ts.client.JoinGame(ctx, &pb.JoinGameRequest{
		UserId: "player-3",
		GameId: gameID,
	})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	// Game not found
	_, err = ts.client.JoinGame(ctx, &pb.JoinGameRequest{
		UserId: "player-3",
		GameId: "nonexistent",
	})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestAcceptance_MakeMove(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create and join a game
	createResp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)

	gameID := createResp.Game.GameId

	_, err = ts.client.JoinGame(ctx, &pb.JoinGameRequest{
		UserId: "player-2",
		GameId: gameID,
	})
	require.NoError(t, err)

	// Player X makes a move
	moveResp, err := ts.client.MakeMove(ctx, &pb.MakeMoveRequest{
		UserId: "player-1",
		GameId: gameID,
		Row:    0,
		Col:    0,
	})
	require.NoError(t, err)

	assert.Equal(t, pb.Mark_MARK_X, moveResp.Game.Board[0])
	assert.Equal(t, pb.Mark_MARK_O, moveResp.Game.CurrentTurn)

	// Player O makes a move
	moveResp, err = ts.client.MakeMove(ctx, &pb.MakeMoveRequest{
		UserId: "player-2",
		GameId: gameID,
		Row:    1,
		Col:    1,
	})
	require.NoError(t, err)

	assert.Equal(t, pb.Mark_MARK_O, moveResp.Game.Board[4]) // 1*3 + 1 = 4
	assert.Equal(t, pb.Mark_MARK_X, moveResp.Game.CurrentTurn)
}

func TestAcceptance_MakeMove_Errors(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create and join a game
	createResp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)

	gameID := createResp.Game.GameId

	// Cannot move before game starts
	_, err = ts.client.MakeMove(ctx, &pb.MakeMoveRequest{
		UserId: "player-1",
		GameId: gameID,
		Row:    0,
		Col:    0,
	})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	_, err = ts.client.JoinGame(ctx, &pb.JoinGameRequest{
		UserId: "player-2",
		GameId: gameID,
	})
	require.NoError(t, err)

	// Wrong turn
	_, err = ts.client.MakeMove(ctx, &pb.MakeMoveRequest{
		UserId: "player-2",
		GameId: gameID,
		Row:    0,
		Col:    0,
	})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	// Make a valid move
	_, err = ts.client.MakeMove(ctx, &pb.MakeMoveRequest{
		UserId: "player-1",
		GameId: gameID,
		Row:    0,
		Col:    0,
	})
	require.NoError(t, err)

	// Cell occupied
	_, err = ts.client.MakeMove(ctx, &pb.MakeMoveRequest{
		UserId: "player-2",
		GameId: gameID,
		Row:    0,
		Col:    0,
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	// Invalid position
	_, err = ts.client.MakeMove(ctx, &pb.MakeMoveRequest{
		UserId: "player-2",
		GameId: gameID,
		Row:    10,
		Col:    10,
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	// Player not in game
	_, err = ts.client.MakeMove(ctx, &pb.MakeMoveRequest{
		UserId: "player-3",
		GameId: gameID,
		Row:    1,
		Col:    1,
	})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestAcceptance_FullGame_XWins(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create and join a game
	createResp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)

	gameID := createResp.Game.GameId

	_, err = ts.client.JoinGame(ctx, &pb.JoinGameRequest{
		UserId: "player-2",
		GameId: gameID,
	})
	require.NoError(t, err)

	// Play a game where X wins
	// X X X
	// O O .
	// . . .
	moves := []struct {
		player string
		row    int32
		col    int32
	}{
		{"player-1", 0, 0},
		{"player-2", 1, 0},
		{"player-1", 0, 1},
		{"player-2", 1, 1},
		{"player-1", 0, 2}, // X wins
	}

	var lastResp *pb.MakeMoveResponse
	for _, m := range moves {
		lastResp, err = ts.client.MakeMove(ctx, &pb.MakeMoveRequest{
			UserId: m.player,
			GameId: gameID,
			Row:    m.row,
			Col:    m.col,
		})
		require.NoError(t, err)
	}

	assert.Equal(t, pb.GameStatus_GAME_STATUS_X_WON, lastResp.Game.Status)

	// Check stats
	statsResp, err := ts.client.GetUserStats(ctx, &pb.GetUserStatsRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), statsResp.Wins)
	assert.Equal(t, int32(0), statsResp.Losses)

	statsResp, err = ts.client.GetUserStats(ctx, &pb.GetUserStatsRequest{
		UserId: "player-2",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(0), statsResp.Wins)
	assert.Equal(t, int32(1), statsResp.Losses)
}

func TestAcceptance_FullGame_Draw(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create and join a game
	createResp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)

	gameID := createResp.Game.GameId

	_, err = ts.client.JoinGame(ctx, &pb.JoinGameRequest{
		UserId: "player-2",
		GameId: gameID,
	})
	require.NoError(t, err)

	// Play a draw game
	// X O X
	// X X O
	// O X O
	moves := []struct {
		player string
		row    int32
		col    int32
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

	var lastResp *pb.MakeMoveResponse
	for _, m := range moves {
		lastResp, err = ts.client.MakeMove(ctx, &pb.MakeMoveRequest{
			UserId: m.player,
			GameId: gameID,
			Row:    m.row,
			Col:    m.col,
		})
		require.NoError(t, err)
	}

	assert.Equal(t, pb.GameStatus_GAME_STATUS_DRAW, lastResp.Game.Status)

	// Check stats
	statsResp, err := ts.client.GetUserStats(ctx, &pb.GetUserStatsRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), statsResp.Draws)

	statsResp, err = ts.client.GetUserStats(ctx, &pb.GetUserStatsRequest{
		UserId: "player-2",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), statsResp.Draws)
}

func TestAcceptance_GetGame(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Create a game
	createResp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)

	gameID := createResp.Game.GameId

	// Get game
	getResp, err := ts.client.GetGame(ctx, &pb.GetGameRequest{
		GameId: gameID,
	})
	require.NoError(t, err)

	assert.Equal(t, gameID, getResp.Game.GameId)
	assert.Equal(t, "player-1", getResp.Game.PlayerXId)

	// Get non-existent game
	_, err = ts.client.GetGame(ctx, &pb.GetGameRequest{
		GameId: "nonexistent",
	})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestAcceptance_GetUserStats(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	// Get stats for new user
	statsResp, err := ts.client.GetUserStats(ctx, &pb.GetUserStatsRequest{
		UserId: "new-user",
	})
	require.NoError(t, err)

	assert.Equal(t, "new-user", statsResp.UserId)
	assert.Equal(t, int32(0), statsResp.Wins)
	assert.Equal(t, int32(0), statsResp.Losses)
	assert.Equal(t, int32(0), statsResp.Draws)
	assert.Equal(t, int32(0), statsResp.TotalGames)
}

func TestAcceptance_StreamGameUpdates(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a game
	createResp, err := ts.client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId: "player-1",
	})
	require.NoError(t, err)

	gameID := createResp.Game.GameId

	// Start streaming
	stream, err := ts.client.StreamGameUpdates(ctx, &pb.StreamGameUpdatesRequest{
		GameId: gameID,
		UserId: "player-1",
	})
	require.NoError(t, err)

	// Receive initial state
	update, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, gameID, update.Game.GameId)
	assert.Equal(t, "Connected to game", update.Message)

	// Join game in another goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)
		ts.client.JoinGame(ctx, &pb.JoinGameRequest{
			UserId: "player-2",
			GameId: gameID,
		})
	}()

	// Receive join update
	update, err = stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, pb.GameStatus_GAME_STATUS_IN_PROGRESS, update.Game.Status)
	assert.Contains(t, update.Message, "started")
}
