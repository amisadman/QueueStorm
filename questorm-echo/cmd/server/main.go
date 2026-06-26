package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"sust-preli/internal/config"
	"sust-preli/internal/engine"
	"sust-preli/internal/queue"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// 1. Load Configuration
	cfg := config.LoadConfig()

	// 2. Initialize LLM Client
	llmClient := engine.NewLLMClient(cfg.GeminiAPIKey, cfg.GeminiAPIKeyBackup, cfg.GroqAPIKey)

	// 3. Initialize Concurrency Worker Pool
	// We can configure workers and queue size from env, or default to reasonable values.
	// E.g., max 10 concurrent workers and queue size of 100.
	maxWorkers := 10
	if envWorkers := os.Getenv("MAX_WORKERS"); envWorkers != "" {
		if val, err := strconv.Atoi(envWorkers); err == nil {
			maxWorkers = val
		}
	}
	workerPool := queue.NewWorkerPool(maxWorkers, 100)

	// Define the core analysis pipeline function
	pipelineFunc := func(ctx context.Context, req *engine.TicketRequest) (*engine.TicketResponse, error) {
		// Tier 1: Run Deterministic Go Rule Engine to extract facts and resolve transaction matching
		facts := engine.AnalyzeDeterministic(req)

		// Tier 2: Run Multi-LLM Failover Chain to generate natural language fields
		resp, err := llmClient.GenerateAnalysis(ctx, req, facts)
		if err != nil {
			return nil, err
		}

		// Tier 4 (Guardrails): Programmatically sanitize outputs for safety compliance
		engine.SanitizeResponse(resp, facts.Language)

		return resp, nil
	}

	// Start the worker pool
	workerPool.Start(pipelineFunc)

	// 4. Set up Echo Server
	e := echo.New()

	// Middlewares
	e.Use(middleware.Logger())
	e.Use(middleware.Recover()) // Prevent crashes on panic

	// HTTP GET /health
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// HTTP POST /analyze-ticket
	e.POST("/analyze-ticket", func(c echo.Context) error {
		// Bind request body
		req := new(engine.TicketRequest)
		if err := c.Bind(req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Malformed input: invalid JSON structure",
			})
		}

		// Strict Validation
		if req.TicketID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Missing required field: ticket_id",
			})
		}
		if req.Complaint == "" || strings.TrimSpace(req.Complaint) == "" {
			// Semantically invalid: empty or whitespace-only complaint is a 422
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error": "Semantically invalid input: complaint text cannot be empty",
			})
		}

		// Enforce strict per-request timeout of 28 seconds (safely under 30 seconds limit)
		ctx, cancel := context.WithTimeout(c.Request().Context(), 28*time.Second)
		defer cancel()

		// Submit request to the concurrency-safe worker pool queue
		resp, err := workerPool.Submit(ctx, req)
		if err != nil {
			if err == context.DeadlineExceeded {
				return c.JSON(http.StatusGatewayTimeout, map[string]string{
					"error": "Request timed out: analysis took too long",
				})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Internal server error: " + err.Error(),
			})
		}

		return c.JSON(http.StatusOK, resp)
	})

	// 5. Start HTTP Server
	serverPort := ":" + cfg.Port
	e.Logger.Fatal(e.Start(serverPort))
}

// Simple helper to avoid import loop or extra imports
func stringsTrimSpace(s string) string {
	return s // handled by Go compiler or custom logic. Actually, we can import strings!
}
