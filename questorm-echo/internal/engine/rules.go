package engine

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// FactAnalysis represents the structured findings of the deterministic rule engine.
type FactAnalysis struct {
	Language              string
	ExtractedAmount       float64
	ExtractedTxnID        string
	MatchedTxn            *Transaction
	Verdict               string
	CaseType              string
	Severity              string
	Department            string
	HumanReviewRequired   bool
	ReasonCodes           []string
	Confidence            float64
	IsDuplicatePayment    bool
	DuplicateTxn          *Transaction
	EstablishedPattern    bool
	IsPhishing            bool
	IsVague               bool
	DetectedBPN           bool // Bangla script detected
}

// Convert Bangla digits to English digits.
func convertBanglaDigits(input string) string {
	var banglaToEnglish = map[rune]rune{
		'০': '0', '১': '1', '২': '2', '৩': '3', '৪': '4',
		'৫': '5', '৬': '6', '৭': '8', '৮': '8', '৯': '9', // Note: User map correction: '৭' is '7', '৮' is '8'
	}
	// Let's fix the map manually for correctness:
	banglaToEnglish['৭'] = '7'
	banglaToEnglish['৮'] = '8'

	runes := []rune(input)
	for i, r := range runes {
		if eng, ok := banglaToEnglish[r]; ok {
			runes[i] = eng
		}
	}
	return string(runes)
}

// Check if string contains Bengali script.
func hasBengali(s string) bool {
	for _, r := range s {
		if r >= 0x0980 && r <= 0x09FF {
			return true
		}
	}
	return false
}

// Normalize text by converting to lowercase and stripping common punctuation.
func normalizeText(text string) string {
	s := convertBanglaDigits(text)
	s = strings.ToLower(s)
	return s
}

// AnalyzeDeterministic processes the ticket request and transaction history
// to extract deterministic facts and make initial classifications.
func AnalyzeDeterministic(req *TicketRequest) *FactAnalysis {
	analysis := &FactAnalysis{
		Language:            "en",
		Verdict:             VerdictInsufficientData,
		CaseType:            CaseOther,
		Severity:            SeverityLow,
		Department:          DeptCustomerSupport,
		HumanReviewRequired: false,
		ReasonCodes:         []string{},
		Confidence:          0.5,
	}

	// 1. Detect Language
	complaintNormal := normalizeText(req.Complaint)
	hasBn := hasBengali(req.Complaint)
	analysis.DetectedBPN = hasBn

	if req.Language != "" {
		analysis.Language = req.Language
	} else if hasBn {
		// Simple heuristic: if it has Bangla and some English words, it could be mixed,
		// but if it's primarily Bangla, set to bn. Let's look for english characters
		hasEn := false
		for _, r := range req.Complaint {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				hasEn = true
				break
			}
		}
		if hasEn && hasBn {
			analysis.Language = "mixed"
		} else if hasBn {
			analysis.Language = "bn"
		}
	}

	// 2. Extract Amount
	// Match numbers like 500, 1000, 5000 etc.
	amountRegex := regexp.MustCompile(`\b\d{3,6}\b`)
	amountMatches := amountRegex.FindAllString(complaintNormal, -1)
	var extractedAmount float64
	if len(amountMatches) > 0 {
		// Take the first or largest number as the potential amount. Let's parse the first.
		if val, err := strconv.ParseFloat(amountMatches[0], 64); err == nil {
			extractedAmount = val
			analysis.ExtractedAmount = val
		}
	}

	// 3. Extract Transaction ID
	// Match patterns like TXN-12345 or TXN12345
	txnRegex := regexp.MustCompile(`(?i)txn-?\d+`)
	txnMatch := txnRegex.FindString(req.Complaint)
	if txnMatch != "" {
		// Normalize transaction ID (ensure uppercase and hyphen format if it was txn10001 -> TXN-10001)
		idNumRegex := regexp.MustCompile(`\d+`)
		idNum := idNumRegex.FindString(txnMatch)
		analysis.ExtractedTxnID = "TXN-" + idNum
	}

	// 4. Check for Phishing/Social Engineering Keywords
	phishingKeywords := []string{
		"otp", "pin", "password", "bKash", "scam", "fraud", "block account", "security code",
		"ওটিপি", "পিন", "পাসওয়ার্ড", "কল", "একাউন্ট ব্লক", "কার্ড নাম্বার",
	}
	isPhishing := false
	for _, kw := range phishingKeywords {
		if strings.Contains(complaintNormal, kw) {
			// If they ask for credentials or mention scam calls, it's likely phishing
			if strings.Contains(complaintNormal, "ask") || strings.Contains(complaintNormal, "call") ||
				strings.Contains(complaintNormal, "চেয়েছে") || strings.Contains(complaintNormal, "বলছে") ||
				strings.Contains(complaintNormal, "দাও") || strings.Contains(complaintNormal, "দেও") {
				isPhishing = true
				break
			}
		}
	}
	analysis.IsPhishing = isPhishing

	// 5. Transaction Matching and Classification Logic
	historyCount := len(req.TransactionHistory)

	// A: Handle Phishing directly
	if isPhishing {
		analysis.IsPhishing = true
		analysis.CaseType = CasePhishingOrSocialEngineering
		analysis.Department = DeptFraudRisk
		analysis.Severity = SeverityCritical
		analysis.Verdict = VerdictInsufficientData // cannot verify a scam caller from transaction ledger
		analysis.HumanReviewRequired = true
		analysis.Confidence = 0.95
		analysis.ReasonCodes = []string{"phishing", "credential_protection", "critical_escalation"}
		return analysis
	}

	// B: Search for Duplicate Payments in History
	// Duplicate payment: two identical completed payments/transfers to the same counterparty within a short window (e.g. 5 minutes)
	if historyCount >= 2 {
		for i := 0; i < historyCount; i++ {
			t1 := req.TransactionHistory[i]
			if t1.Status != "completed" || (t1.Type != "payment" && t1.Type != "transfer") {
				continue
			}
			for j := i + 1; j < historyCount; j++ {
				t2 := req.TransactionHistory[j]
				if t2.Status != "completed" || t2.Type != t1.Type || t2.Amount != t1.Amount || t2.Counterparty != t1.Counterparty {
					continue
				}

				// Parse timestamps and compare
				time1, err1 := time.Parse(time.RFC3339, t1.Timestamp)
				time2, err2 := time.Parse(time.RFC3339, t2.Timestamp)
				if err1 == nil && err2 == nil {
					diff := math.Abs(time1.Sub(time2).Minutes())
					if diff <= 5.0 {
						analysis.IsDuplicatePayment = true
						// The duplicate transaction is usually the second one (more recent timestamp)
						if time1.After(time2) {
							analysis.MatchedTxn = &t1
							analysis.DuplicateTxn = &t2
						} else {
							analysis.MatchedTxn = &t2
							analysis.DuplicateTxn = &t1
						}
					}
				}
			}
		}
	}

	// C: If duplicate payment found and complaint contains duplicate keywords
	duplicateKeywords := []string{"twice", "double", "two times", "deducted twice", "charged twice", "দুইবার", "২ বার", "কেটেছে", "ডাবল"}
	hasDuplicateKw := false
	for _, kw := range duplicateKeywords {
		if strings.Contains(complaintNormal, kw) {
			hasDuplicateKw = true
			break
		}
	}

	if analysis.IsDuplicatePayment && (hasDuplicateKw || extractedAmount == analysis.MatchedTxn.Amount) {
		analysis.CaseType = CaseDuplicatePayment
		analysis.Verdict = VerdictConsistent
		analysis.Department = DeptPaymentsOps
		analysis.Severity = SeverityHigh
		analysis.HumanReviewRequired = true
		analysis.Confidence = 0.95
		analysis.ReasonCodes = []string{"duplicate_payment", "biller_verification_required"}
		return analysis
	}

	// D: Standard transaction matching (find a transaction that matches extracted amount or txn ID)
	var matchedTransactions []Transaction
	for _, t := range req.TransactionHistory {
		// Match by exact Txn ID
		if analysis.ExtractedTxnID != "" && strings.EqualFold(t.TransactionID, analysis.ExtractedTxnID) {
			matchedTransactions = append(matchedTransactions, t)
			break // exact match found
		}
		// Match by amount
		if extractedAmount > 0 && t.Amount == extractedAmount {
			matchedTransactions = append(matchedTransactions, t)
		}
	}

	// E: Classify based on matched transactions
	if len(matchedTransactions) == 1 {
		t := matchedTransactions[0]
		analysis.MatchedTxn = &t
		analysis.Verdict = VerdictConsistent
		analysis.Confidence = 0.9

		// Check intent from complaint (precise wrong transfer claims)
		isWrongTransfer := strings.Contains(complaintNormal, "wrong transfer") ||
			strings.Contains(complaintNormal, "wrong number") ||
			strings.Contains(complaintNormal, "wrong recipient") ||
			strings.Contains(complaintNormal, "wrong person") ||
			strings.Contains(complaintNormal, "typing") ||
			strings.Contains(complaintNormal, "sent to the wrong") ||
			strings.Contains(complaintNormal, "send to the wrong") ||
			strings.Contains(complaintNormal, "ভুল নম্বর") ||
			strings.Contains(complaintNormal, "ভুল নাম্বার") ||
			strings.Contains(complaintNormal, "ভুল নাম্বারে") ||
			strings.Contains(complaintNormal, "ভুল নম্বরে")
		isFailed := strings.Contains(complaintNormal, "failed") || strings.Contains(complaintNormal, "ব্যর্থ") || strings.Contains(complaintNormal, "কেটেছে")
		isRefund := strings.Contains(complaintNormal, "refund") || strings.Contains(complaintNormal, "mind") || strings.Contains(complaintNormal, "ফেরত") || strings.Contains(complaintNormal, "রিফান্ড")
		isAgentCashIn := strings.Contains(complaintNormal, "agent") || strings.Contains(complaintNormal, "cash") || strings.Contains(complaintNormal, "এজেন্ট")

		if isWrongTransfer || t.Type == "transfer" && (isWrongTransfer || !isFailed && !isRefund) {
			analysis.CaseType = CaseWrongTransfer
			analysis.Department = DeptDisputeResolution
			analysis.Severity = SeverityHigh
			analysis.HumanReviewRequired = true

			// Check for "Established Pattern" (Inconsistent evidence)
			// If there are other completed transfers/payments to the same counterparty in history, it contradicts a "wrong transfer" claim.
			patternCount := 0
			for _, prev := range req.TransactionHistory {
				if prev.TransactionID != t.TransactionID && prev.Counterparty == t.Counterparty && prev.Status == "completed" {
					patternCount++
				}
			}
			if patternCount > 0 {
				analysis.EstablishedPattern = true
				analysis.Verdict = VerdictInconsistent
				analysis.Severity = SeverityMedium // downgraded from high to medium as per sample 2
				analysis.ReasonCodes = []string{"wrong_transfer_claim", "established_recipient_pattern", "evidence_inconsistent"}
			} else {
				analysis.ReasonCodes = []string{"wrong_transfer", "transaction_match", "dispute_initiated"}
			}

		} else if t.Type == "payment" && t.Status == "failed" && (isFailed || strings.Contains(complaintNormal, "deducted")) {
			analysis.CaseType = CasePaymentFailed
			analysis.Department = DeptPaymentsOps
			analysis.Severity = SeverityHigh
			analysis.HumanReviewRequired = false // standard failed payments don't require human review immediately if auto-reversed
			analysis.ReasonCodes = []string{"payment_failed", "potential_balance_deduction"}

		} else if isRefund || (t.Type == "payment" && isRefund) {
			analysis.CaseType = CaseRefundRequest
			analysis.Department = DeptCustomerSupport
			analysis.Severity = SeverityLow
			analysis.HumanReviewRequired = false
			analysis.ReasonCodes = []string{"refund_request", "merchant_policy_dependent"}

		} else if t.Type == "settlement" {
			analysis.CaseType = CaseMerchantSettlementDelay
			analysis.Department = DeptMerchantOperations
			if t.Status == "pending" {
				analysis.Verdict = VerdictConsistent
				analysis.Severity = SeverityMedium
				analysis.ReasonCodes = []string{"merchant_settlement", "delay", "pending"}
			} else {
				analysis.Verdict = VerdictInconsistent
				analysis.Severity = SeverityLow
			}

		} else if t.Type == "cash_in" && (isAgentCashIn || strings.Contains(complaintNormal, "balance")) {
			analysis.CaseType = CaseAgentCashInIssue
			analysis.Department = DeptAgentOperations
			analysis.HumanReviewRequired = true
			if t.Status == "pending" {
				analysis.Verdict = VerdictConsistent
				analysis.Severity = SeverityHigh
				analysis.ReasonCodes = []string{"agent_cash_in", "pending_transaction", "agent_ops"}
			} else {
				analysis.Verdict = VerdictInconsistent
				analysis.Severity = SeverityMedium
			}
		} else {
			// Fallback classification based on transaction type
			analysis.CaseType = CaseOther
			analysis.Department = DeptCustomerSupport
			analysis.Severity = SeverityLow
			analysis.ReasonCodes = []string{"transaction_matched", "other_category"}
		}

	} else if len(matchedTransactions) > 1 {
		// Multiple ambiguous transactions match the complaint (like SAMPLE-08)
		analysis.Verdict = VerdictInsufficientData
		analysis.Confidence = 0.65
		analysis.HumanReviewRequired = false

		// Check type of transactions to guess case type
		firstTxn := matchedTransactions[0]
		if firstTxn.Type == "transfer" {
			analysis.CaseType = CaseWrongTransfer
			analysis.Department = DeptDisputeResolution
			analysis.Severity = SeverityMedium
		} else {
			analysis.CaseType = CaseOther
			analysis.Department = DeptCustomerSupport
			analysis.Severity = SeverityLow
		}
		analysis.ReasonCodes = []string{"ambiguous_match", "needs_clarification"}

	} else {
		// No transactions matched
		analysis.Verdict = VerdictInsufficientData
		analysis.HumanReviewRequired = false
		analysis.Confidence = 0.6

		// Classify based on text keywords if no transactions matched (precise wrong transfer claims)
		isWrongTransferClaim := strings.Contains(complaintNormal, "wrong transfer") ||
			strings.Contains(complaintNormal, "wrong number") ||
			strings.Contains(complaintNormal, "wrong recipient") ||
			strings.Contains(complaintNormal, "wrong person") ||
			strings.Contains(complaintNormal, "typing") ||
			strings.Contains(complaintNormal, "sent to the wrong") ||
			strings.Contains(complaintNormal, "send to the wrong") ||
			strings.Contains(complaintNormal, "ভুল নম্বর") ||
			strings.Contains(complaintNormal, "ভুল নাম্বার") ||
			strings.Contains(complaintNormal, "ভুল নাম্বারে") ||
			strings.Contains(complaintNormal, "ভুল নম্বরে")

		if isWrongTransferClaim {
			analysis.CaseType = CaseWrongTransfer
			analysis.Department = DeptDisputeResolution
			analysis.Severity = SeverityMedium
			analysis.ReasonCodes = []string{"vague_complaint", "wrong_transfer_unmatched"}
		} else {
			analysis.IsVague = true
			analysis.CaseType = CaseOther
			analysis.Department = DeptCustomerSupport
			analysis.Severity = SeverityLow
			analysis.ReasonCodes = []string{"vague_complaint", "needs_clarification"}
		}
	}

	return analysis
}
