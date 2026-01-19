package acceptance

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "tictactoe/api/gen/tictactoe"
)

// LoadTestResult holds the results of the load test
type LoadTestResult struct {
	TotalGames      int
	CompletedGames  int32
	XWins           int32
	OWins           int32
	Draws           int32
	Errors          int32
	TotalMoves      int32
	TotalDuration   time.Duration
	AvgGameDuration time.Duration
	GamesPerSecond  float64
	MovesPerSecond  float64
	ConcurrentUsers int
}

func (r LoadTestResult) String() string {
	return fmt.Sprintf(`
================================================================================
                         LOAD TEST RESULTS
================================================================================
Configuration:
  - Concurrent Users:     %d
  - Total Games:          %d

Results:
  - Completed Games:      %d
  - X Wins:               %d (%.1f%%)
  - O Wins:               %d (%.1f%%)
  - Draws:                %d (%.1f%%)
  - Errors:               %d

Performance:
  - Total Duration:       %v
  - Avg Game Duration:    %v
  - Games/Second:         %.2f
  - Moves/Second:         %.2f
  - Total Moves:          %d
================================================================================
`,
		r.ConcurrentUsers,
		r.TotalGames,
		r.CompletedGames,
		r.XWins, float64(r.XWins)/float64(r.CompletedGames)*100,
		r.OWins, float64(r.OWins)/float64(r.CompletedGames)*100,
		r.Draws, float64(r.Draws)/float64(r.CompletedGames)*100,
		r.Errors,
		r.TotalDuration,
		r.AvgGameDuration,
		r.GamesPerSecond,
		r.MovesPerSecond,
		r.TotalMoves,
	)
}

// TestLoadTest_100Users_100Games runs a load test with 1000+ games running truly in parallel
func TestLoadTest_100Users_100Games(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	const (
		numGames  = 1000 // 1000 games running in parallel
		numUsers  = 2000 // 2 users per game
		boardSize = 3
		winLength = 3
	)

	result := LoadTestResult{
		TotalGames:      numGames,
		ConcurrentUsers: numUsers,
	}

	// Create user IDs
	users := make([]string, numUsers)
	for i := 0; i < numUsers; i++ {
		users[i] = fmt.Sprintf("user-%d", i)
	}

	var wg sync.WaitGroup
	startTime := time.Now()

	// Start ALL 1000 games simultaneously (truly parallel)
	for gameNum := 0; gameNum < numGames; gameNum++ {
		wg.Add(1)
		go func(gameNum int) {
			defer wg.Done()

			// Each game has 2 unique users
			playerX := users[gameNum*2]
			playerO := users[gameNum*2+1]

			moves, outcome, err := playFullGame(ctx, ts.client, playerX, playerO, boardSize, winLength)
			atomic.AddInt32(&result.TotalMoves, int32(moves))

			if err != nil {
				atomic.AddInt32(&result.Errors, 1)
				t.Logf("Game %d error: %v", gameNum, err)
				return
			}

			atomic.AddInt32(&result.CompletedGames, 1)

			switch outcome {
			case pb.GameStatus_GAME_STATUS_X_WON:
				atomic.AddInt32(&result.XWins, 1)
			case pb.GameStatus_GAME_STATUS_O_WON:
				atomic.AddInt32(&result.OWins, 1)
			case pb.GameStatus_GAME_STATUS_DRAW:
				atomic.AddInt32(&result.Draws, 1)
			}
		}(gameNum)
	}

	wg.Wait()
	result.TotalDuration = time.Since(startTime)

	// Calculate metrics
	if result.CompletedGames > 0 {
		result.AvgGameDuration = result.TotalDuration / time.Duration(result.CompletedGames)
		result.GamesPerSecond = float64(result.CompletedGames) / result.TotalDuration.Seconds()
		result.MovesPerSecond = float64(result.TotalMoves) / result.TotalDuration.Seconds()
	}

	// Print results
	t.Log(result.String())

	// Verify user stats - sample from different ranges
	t.Log("\n=== Sample User Statistics ===")
	sampleUsers := []int{0, 1, 100, 101, 500, 501, 998, 999, 1998, 1999}
	for _, i := range sampleUsers {
		if i < numUsers {
			stats, err := ts.client.GetUserStats(ctx, &pb.GetUserStatsRequest{UserId: users[i]})
			require.NoError(t, err)
			t.Logf("  %s: W=%d L=%d D=%d Total=%d",
				stats.UserId, stats.Wins, stats.Losses, stats.Draws, stats.TotalGames)
		}
	}

	// Assertions
	assert.Equal(t, int32(0), result.Errors, "Should have no errors")
	assert.Equal(t, int32(numGames), result.CompletedGames, "All games should complete")
	assert.Equal(t, result.CompletedGames, result.XWins+result.OWins+result.Draws, "All games should have an outcome")
}

// TestLoadTest_HighConcurrency tests with very high concurrency
func TestLoadTest_HighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	const (
		numGames       = 200
		workerPoolSize = 100 // High concurrency
	)

	var (
		completedGames int32
		errors         int32
		totalMoves     int32
	)

	gamesChan := make(chan int, numGames)
	for i := 0; i < numGames; i++ {
		gamesChan <- i
	}
	close(gamesChan)

	var wg sync.WaitGroup
	startTime := time.Now()

	for w := 0; w < workerPoolSize; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for gameNum := range gamesChan {
				playerX := fmt.Sprintf("player-x-%d", gameNum)
				playerO := fmt.Sprintf("player-o-%d", gameNum)

				moves, _, err := playFullGame(ctx, ts.client, playerX, playerO, 3, 3)
				atomic.AddInt32(&totalMoves, int32(moves))

				if err != nil {
					atomic.AddInt32(&errors, 1)
				} else {
					atomic.AddInt32(&completedGames, 1)
				}
			}
		}(w)
	}

	wg.Wait()
	duration := time.Since(startTime)

	t.Logf(`
================================================================================
                    HIGH CONCURRENCY TEST RESULTS
================================================================================
  - Games:            %d
  - Workers:          %d
  - Completed:        %d
  - Errors:           %d
  - Duration:         %v
  - Games/Second:     %.2f
  - Moves/Second:     %.2f
================================================================================
`,
		numGames, workerPoolSize, completedGames, errors, duration,
		float64(completedGames)/duration.Seconds(),
		float64(totalMoves)/duration.Seconds(),
	)

	assert.Equal(t, int32(0), errors, "Should have no errors")
	assert.Equal(t, int32(numGames), completedGames, "All games should complete")
}

// TestLoadTest_MixedBoardSizes tests with various board sizes
func TestLoadTest_MixedBoardSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	ts := setupTestServer(t)
	defer ts.cleanup()

	ctx := context.Background()

	boardConfigs := []struct {
		size      int
		winLength int
		count     int
	}{
		{3, 3, 50}, // Standard 3x3
		{4, 3, 30}, // 4x4 with 3 to win
		{5, 4, 20}, // 5x5 with 4 to win
		{6, 4, 10}, // 6x6 with 4 to win
		{7, 5, 5},  // 7x7 with 5 to win
	}

	type configResult struct {
		config    string
		completed int32
		errors    int32
		moves     int32
		duration  time.Duration
	}

	results := make([]configResult, len(boardConfigs))

	var wg sync.WaitGroup

	for idx, cfg := range boardConfigs {
		wg.Add(1)
		go func(idx int, size, winLen, count int) {
			defer wg.Done()

			var completed, errors, moves int32
			start := time.Now()

			var innerWg sync.WaitGroup
			for i := 0; i < count; i++ {
				innerWg.Add(1)
				go func(gameNum int) {
					defer innerWg.Done()

					playerX := fmt.Sprintf("user-x-%d-%d", size, gameNum)
					playerO := fmt.Sprintf("user-o-%d-%d", size, gameNum)

					m, _, err := playFullGame(ctx, ts.client, playerX, playerO, int32(size), int32(winLen))
					atomic.AddInt32(&moves, int32(m))

					if err != nil {
						atomic.AddInt32(&errors, 1)
					} else {
						atomic.AddInt32(&completed, 1)
					}
				}(i)
			}
			innerWg.Wait()

			results[idx] = configResult{
				config:    fmt.Sprintf("%dx%d (win=%d)", size, size, winLen),
				completed: completed,
				errors:    errors,
				moves:     moves,
				duration:  time.Since(start),
			}
		}(idx, cfg.size, cfg.winLength, cfg.count)
	}

	wg.Wait()

	t.Log(`
================================================================================
                    MIXED BOARD SIZES TEST RESULTS
================================================================================`)
	t.Logf("%-20s %10s %10s %10s %15s %15s", "Config", "Completed", "Errors", "Moves", "Duration", "Games/sec")
	t.Log("--------------------------------------------------------------------------------")

	var totalCompleted, totalErrors, totalMoves int32
	for _, r := range results {
		gps := float64(r.completed) / r.duration.Seconds()
		t.Logf("%-20s %10d %10d %10d %15v %15.2f", r.config, r.completed, r.errors, r.moves, r.duration, gps)
		totalCompleted += r.completed
		totalErrors += r.errors
		totalMoves += r.moves
	}
	t.Log("--------------------------------------------------------------------------------")
	t.Logf("%-20s %10d %10d %10d", "TOTAL", totalCompleted, totalErrors, totalMoves)
	t.Log("================================================================================")

	assert.Equal(t, int32(0), totalErrors, "Should have no errors")
}

// playFullGame plays a complete game and returns the number of moves, outcome, and any error
func playFullGame(ctx context.Context, client pb.TicTacToeServiceClient, playerX, playerO string, boardSize, winLength int32) (int, pb.GameStatus, error) {
	// Create game
	createResp, err := client.CreateGame(ctx, &pb.CreateGameRequest{
		UserId:    playerX,
		BoardSize: boardSize,
		WinLength: winLength,
	})
	if err != nil {
		return 0, pb.GameStatus_GAME_STATUS_UNSPECIFIED, fmt.Errorf("create game: %w", err)
	}

	gameID := createResp.Game.GameId

	// Join game
	_, err = client.JoinGame(ctx, &pb.JoinGameRequest{
		UserId: playerO,
		GameId: gameID,
	})
	if err != nil {
		return 0, pb.GameStatus_GAME_STATUS_UNSPECIFIED, fmt.Errorf("join game: %w", err)
	}

	// Play the game with random moves
	moves := 0
	currentPlayer := playerX
	size := int(boardSize)

	// Track available cells
	available := make([]struct{ row, col int }, 0, size*size)
	for r := 0; r < size; r++ {
		for c := 0; c < size; c++ {
			available = append(available, struct{ row, col int }{r, c})
		}
	}

	// Shuffle for randomness
	rand.Shuffle(len(available), func(i, j int) {
		available[i], available[j] = available[j], available[i]
	})

	for i := 0; i < len(available); i++ {
		cell := available[i]

		resp, err := client.MakeMove(ctx, &pb.MakeMoveRequest{
			UserId: currentPlayer,
			GameId: gameID,
			Row:    int32(cell.row),
			Col:    int32(cell.col),
		})
		if err != nil {
			return moves, pb.GameStatus_GAME_STATUS_UNSPECIFIED, fmt.Errorf("make move: %w", err)
		}

		moves++

		// Check if game is over
		if resp.Game.Status != pb.GameStatus_GAME_STATUS_IN_PROGRESS {
			return moves, resp.Game.Status, nil
		}

		// Switch player
		if currentPlayer == playerX {
			currentPlayer = playerO
		} else {
			currentPlayer = playerX
		}
	}

	// Should not reach here for a valid game
	return moves, pb.GameStatus_GAME_STATUS_DRAW, nil
}
