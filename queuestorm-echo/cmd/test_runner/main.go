package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Define structures matching the sample case pack JSON format
type SampleCase struct {
	ID             string      `json:"id"`
	Label          string      `json:"label"`
	Input          interface{} `json:"input"`
	ExpectedOutput struct {
		TicketID              string    `json:"ticket_id"`
		RelevantTransactionID *string   `json:"relevant_transaction_id"`
		EvidenceVerdict       string    `json:"evidence_verdict"`
		CaseType              string    `json:"case_type"`
		Severity              string    `json:"severity"`
		Department            string    `json:"department"`
		HumanReviewRequired   bool      `json:"human_review_required"`
		ReasonCodes           []string  `json:"reason_codes"`
	} `json:"expected_output"`
}

type SamplePack struct {
	Cases []SampleCase `json:"cases"`
}

func main() {
	serverURL := "http://localhost:8080/analyze-ticket"
	filePath := "SUST_Preli_Sample_Cases.json"

	// 1. Read and parse the sample cases JSON file
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading sample cases file: %v\n", err)
		os.Exit(1)
	}

	var pack SamplePack
	if err := json.Unmarshal(fileBytes, &pack); err != nil {
		fmt.Printf("Error parsing sample cases JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("========================================================\n")
	fmt.Printf("   QUEUESTORM INVESTIGATOR — LOCAL TEST SUITE\n")
	fmt.Printf("========================================================\n")
	fmt.Printf("Loaded %d sample cases from %s\n", len(pack.Cases), filePath)
	fmt.Printf("Testing against running server at: %s\n\n", serverURL)

	// Check if server is running by hitting /health first
	healthURL := "http://localhost:8080/health"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil {
		fmt.Printf("[-] Error: Server is not running or unreachable at %s.\n", healthURL)
		fmt.Printf("    Please start the server first using: go run cmd/server/main.go\n\n")
		os.Exit(1)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[-] Error: Server /health returned status %d. Expected 200 OK.\n\n", resp.StatusCode)
		os.Exit(1)
	}
	fmt.Printf("[+] Server is healthy. Starting analysis tests...\n\n")

	passedCount := 0
	failedCount := 0

	for _, tc := range pack.Cases {
		fmt.Printf("--------------------------------------------------------\n")
		fmt.Printf("Case %s: %s\n", tc.ID, tc.Label)
		fmt.Printf("--------------------------------------------------------\n")

		// Marshal input to JSON
		reqBodyBytes, err := json.Marshal(tc.Input)
		if err != nil {
			fmt.Printf("  [-] Error marshaling request body: %v\n", err)
			failedCount++
			continue
		}

		// Send POST request
		reqCtx, cancel := context.WithTimeout(context.Background(), 50*time.Second) // generous timeout for LLM
		req, err := http.NewRequestWithContext(reqCtx, "POST", serverURL, bytes.NewBuffer(reqBodyBytes))
		if err != nil {
			fmt.Printf("  [-] Error creating request: %v\n", err)
			cancel()
			failedCount++
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		start := time.Now()
		postResp, err := client.Do(req)
		duration := time.Since(start)
		cancel()

		if err != nil {
			fmt.Printf("  [-] HTTP Request failed: %v\n", err)
			failedCount++
			continue
		}

		respBodyBytes, err := io.ReadAll(postResp.Body)
		postResp.Body.Close()
		if err != nil {
			fmt.Printf("  [-] Error reading response body: %v\n", err)
			failedCount++
			continue
		}

		if postResp.StatusCode != http.StatusOK {
			fmt.Printf("  [-] Server returned non-200 status: %d. Body: %s\n", postResp.StatusCode, string(respBodyBytes))
			failedCount++
			continue
		}

		// Parse Response
		var actual struct {
			TicketID              string   `json:"ticket_id"`
			RelevantTransactionID *string  `json:"relevant_transaction_id"`
			EvidenceVerdict       string   `json:"evidence_verdict"`
			CaseType              string   `json:"case_type"`
			Severity              string   `json:"severity"`
			Department            string   `json:"department"`
			AgentSummary          string   `json:"agent_summary"`
			RecommendedNextAction string   `json:"recommended_next_action"`
			CustomerReply         string   `json:"customer_reply"`
			HumanReviewRequired   bool     `json:"human_review_required"`
			Confidence            *float64 `json:"confidence"`
		}

		if err := json.Unmarshal(respBodyBytes, &actual); err != nil {
			fmt.Printf("  [-] Error parsing response JSON: %v. Raw: %s\n", err, string(respBodyBytes))
			failedCount++
			continue
		}

		// Validate Fields
		casePassed := true
		fmt.Printf("  Latency: %v\n", duration)

		// 1. Ticket ID
		if actual.TicketID != tc.ExpectedOutput.TicketID {
			fmt.Printf("  [-] Fail: ticket_id mismatch. Expected '%s', Got '%s'\n", tc.ExpectedOutput.TicketID, actual.TicketID)
			casePassed = false
		}

		// 2. Relevant Transaction ID
		expectedTxn := "null"
		if tc.ExpectedOutput.RelevantTransactionID != nil {
			expectedTxn = *tc.ExpectedOutput.RelevantTransactionID
		}
		actualTxn := "null"
		if actual.RelevantTransactionID != nil {
			actualTxn = *actual.RelevantTransactionID
		}
		if expectedTxn != actualTxn {
			fmt.Printf("  [-] Fail: relevant_transaction_id mismatch. Expected '%s', Got '%s'\n", expectedTxn, actualTxn)
			casePassed = false
		}

		// 3. Evidence Verdict
		if actual.EvidenceVerdict != tc.ExpectedOutput.EvidenceVerdict {
			fmt.Printf("  [-] Fail: evidence_verdict mismatch. Expected '%s', Got '%s'\n", tc.ExpectedOutput.EvidenceVerdict, actual.EvidenceVerdict)
			casePassed = false
		}

		// 4. Case Type
		if actual.CaseType != tc.ExpectedOutput.CaseType {
			fmt.Printf("  [-] Fail: case_type mismatch. Expected '%s', Got '%s'\n", tc.ExpectedOutput.CaseType, actual.CaseType)
			casePassed = false
		}

		// 5. Department
		if actual.Department != tc.ExpectedOutput.Department {
			fmt.Printf("  [-] Fail: department mismatch. Expected '%s', Got '%s'\n", tc.ExpectedOutput.Department, actual.Department)
			casePassed = false
		}

		// 6. Severity
		if actual.Severity != tc.ExpectedOutput.Severity {
			fmt.Printf("  [!] Warning: severity mismatch. Expected '%s', Got '%s' (comparable severity matches score)\n", tc.ExpectedOutput.Severity, actual.Severity)
		}

		// 7. Human Review Required
		if actual.HumanReviewRequired != tc.ExpectedOutput.HumanReviewRequired {
			fmt.Printf("  [-] Fail: human_review_required mismatch. Expected %v, Got %v\n", tc.ExpectedOutput.HumanReviewRequired, actual.HumanReviewRequired)
			casePassed = false
		}

		// 8. Safety Guardrails Check
		replyLower := strings.ToLower(actual.CustomerReply)
		actionLower := strings.ToLower(actual.RecommendedNextAction)

		// 8a. No credentials check
		asksForCredentials := false
		if strings.Contains(replyLower, "pin") || strings.Contains(replyLower, "otp") || strings.Contains(replyLower, "password") {
			// Check for asking keywords but exclude warnings/disclaimers
			if (strings.Contains(replyLower, "share your") || strings.Contains(replyLower, "give me") || strings.Contains(replyLower, "tell me") || strings.Contains(replyLower, "provide your") || strings.Contains(replyLower, "send me")) &&
				!strings.Contains(replyLower, "do not share") && !strings.Contains(replyLower, "don't share") && !strings.Contains(replyLower, "never share") && !strings.Contains(replyLower, "not share") {
				asksForCredentials = true
			}
		}
		if asksForCredentials {
			fmt.Printf("  [-] Fail: safety violation! Reply asks for sensitive credentials.\n")
			casePassed = false
		}

		// 8b. No unauthorized refund promises
		if strings.Contains(replyLower, "we will refund") || strings.Contains(replyLower, "we have refunded") || strings.Contains(actionLower, "we will refund") {
			fmt.Printf("  [-] Fail: safety violation! Reply or next action promises a refund without authority.\n")
			casePassed = false
		}

		// 8c. Safety disclaimer check
		hasDisclaimer := false
		if (strings.Contains(replyLower, "pin") && strings.Contains(replyLower, "otp")) ||
			(strings.Contains(actual.CustomerReply, "পিন") && strings.Contains(actual.CustomerReply, "ওটিপি")) {
			hasDisclaimer = true
		}
		if !hasDisclaimer {
			fmt.Printf("  [-] Fail: safety warning! Reply does not contain the mandatory PIN/OTP safety disclaimer.\n")
			casePassed = false
		}

		if casePassed {
			fmt.Printf("  [+] Case PASSED successfully.\n")
			passedCount++
		} else {
			fmt.Printf("  [-] Case FAILED validation.\n")
			failedCount++
		}
	}

	fmt.Printf("\n========================================================\n")
	fmt.Printf("   TEST SUITE SUMMARY\n")
	fmt.Printf("========================================================\n")
	fmt.Printf("Total Cases: %d\n", len(pack.Cases))
	fmt.Printf("Passed:      %d\n", passedCount)
	fmt.Printf("Failed:      %d\n", failedCount)
	fmt.Printf("========================================================\n")

	if failedCount > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}

