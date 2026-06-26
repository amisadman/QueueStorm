package engine

// Transaction represents an entry in the customer's transaction history.
type Transaction struct {
	TransactionID string  `json:"transaction_id"`
	Timestamp     string  `json:"timestamp"`
	Type          string  `json:"type"` // transfer, payment, cash_in, cash_out, settlement, refund
	Amount        float64 `json:"amount"`
	Counterparty  string  `json:"counterparty"`
	Status        string  `json:"status"` // completed, failed, pending, reversed
}

// TicketRequest represents the incoming payload for POST /analyze-ticket.
type TicketRequest struct {
	TicketID           string        `json:"ticket_id"`
	Complaint          string        `json:"complaint"`
	Language           string        `json:"language,omitempty"`            // en, bn, mixed
	Channel            string        `json:"channel,omitempty"`             // in_app_chat, call_center, email, merchant_portal, field_agent
	UserType           string        `json:"user_type,omitempty"`           // customer, merchant, agent, unknown
	CampaignContext    string        `json:"campaign_context,omitempty"`    // optional context
	TransactionHistory []Transaction `json:"transaction_history,omitempty"` // recent transactions
	Metadata           interface{}   `json:"metadata,omitempty"`            // optional metadata
}

// TicketResponse represents the output payload from the analysis.
type TicketResponse struct {
	TicketID              string    `json:"ticket_id"`
	RelevantTransactionID *string   `json:"relevant_transaction_id"` // string or null
	EvidenceVerdict       string    `json:"evidence_verdict"`        // consistent, inconsistent, insufficient_data
	CaseType              string    `json:"case_type"`               // wrong_transfer, payment_failed, etc.
	Severity              string    `json:"severity"`                // low, medium, high, critical
	Department            string    `json:"department"`              // dispute_resolution, customer_support, etc.
	AgentSummary          string    `json:"agent_summary"`
	RecommendedNextAction string    `json:"recommended_next_action"`
	CustomerReply         string    `json:"customer_reply"`
	HumanReviewRequired   bool      `json:"human_review_required"`
	Confidence            *float64  `json:"confidence,omitempty"`   // optional
	ReasonCodes           []string  `json:"reason_codes,omitempty"` // optional
}

// Enum Constants for strict compliance
const (
	// Evidence Verdicts
	VerdictConsistent       = "consistent"
	VerdictInconsistent     = "inconsistent"
	VerdictInsufficientData = "insufficient_data"

	// Case Types
	CaseWrongTransfer              = "wrong_transfer"
	CasePaymentFailed              = "payment_failed"
	CaseRefundRequest              = "refund_request"
	CaseDuplicatePayment           = "duplicate_payment"
	CaseMerchantSettlementDelay    = "merchant_settlement_delay"
	CaseAgentCashInIssue           = "agent_cash_in_issue"
	CasePhishingOrSocialEngineering = "phishing_or_social_engineering"
	CaseOther                      = "other"

	// Severities
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"

	// Departments
	DeptCustomerSupport    = "customer_support"
	DeptDisputeResolution  = "dispute_resolution"
	DeptPaymentsOps        = "payments_ops"
	DeptMerchantOperations = "merchant_operations"
	DeptAgentOperations    = "agent_operations"
	DeptFraudRisk          = "fraud_risk"
)
