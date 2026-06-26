package engine

import (
	"regexp"
	"strings"
)

// SanitizeResponse post-processes the generated response fields (customer_reply and recommended_next_action)
// to strictly enforce the hackathon safety rules and prevent point deductions.
func SanitizeResponse(resp *TicketResponse, detectedLang string) {
	if resp == nil {
		return
	}

	// 1. Sanitize Customer Reply and Recommended Action for refund promises
	resp.CustomerReply = sanitizeRefundPromises(resp.CustomerReply, detectedLang)
	resp.RecommendedNextAction = sanitizeRefundPromises(resp.RecommendedNextAction, detectedLang)

	// 2. Sanitize Customer Reply for credential requests (PIN, OTP, password)
	resp.CustomerReply = sanitizeCredentialRequests(resp.CustomerReply, detectedLang)

	// 3. Block third-party suspicious contact instructions in Customer Reply
	resp.CustomerReply = sanitizeThirdPartyContacts(resp.CustomerReply)

	// 4. Ensure safety disclaimer is present in Customer Reply
	ensureSafetyDisclaimer(resp, detectedLang)
}

// Replaces direct refund commitments ("we will refund you") with safe authorized language
func sanitizeRefundPromises(text string, lang string) string {
	// Forbidden English patterns
	refundPatternsEn := []string{
		`(?i)\bwe\s+will\s+refund\s+you\b`,
		`(?i)\bi\s+will\s+refund\s+you\b`,
		`(?i)\bwe\s+have\s+refunded\s+you\b`,
		`(?i)\bwe\s+will\s+reverse\s+it\b`,
		`(?i)\bwe\s+will\s+reverse\s+your\b`,
		`(?i)\bi\s+will\s+reverse\s+your\b`,
		`(?i)\brefund\s+your\s+money\b`,
		`(?i)\brefund\s+is\s+processed\b`,
		`(?i)\bwe\s+are\s+refunding\b`,
		`(?i)\bmoney\s+will\s+be\s+refunded\b`,
	}

	// Forbidden Bangla patterns
	refundPatternsBn := []string{
		`আমরা\s+আপনার\s+টাকা\s+ফেরত\s+দিব`,
		`আমরা\s+টাকা\s+ফেরত\s+দিচ্ছি`,
		`আমরা\s+ফেরত\s+দিব`,
		`রিফান্ড\s+করে\s+দিব`,
		`রিফান্ড\s+দেওয়া\s+হবে`,
		`টাকা\s+ফেরত\s+পাবেন`,
	}

	safeEn := "any eligible amount will be returned through official channels"
	safeBn := "যেকোনো যোগ্য পরিমাণ অফিসিয়াল চ্যানেলের মাধ্যমে ফেরত দেওয়া হবে"

	result := text
	for _, pattern := range refundPatternsEn {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, safeEn)
	}

	for _, pattern := range refundPatternsBn {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, safeBn)
	}

	return result
}

// Identifies and blocks requests for sensitive credentials (PIN, OTP, password)
func sanitizeCredentialRequests(text string, lang string) string {
	// If the reply asks the customer for PIN, OTP or passwords, we must intercept.
	// E.g. "please share your pin", "send me the otp", "পিন শেয়ার করুন"
	askPatternsEn := []string{
		`(?i)\b(?:share|send|provide|give|tell|enter)\s+(?:your|me|us)?\s*(?:pin|otp|password|cvv|card\s*number)\b`,
		`(?i)\b(?:ask\s+for|request)\s+(?:your|me|us)?\s*(?:pin|otp|password|cvv|card\s*number)\b`,
	}

	askPatternsBn := []string{
		`পিন\s*(?:শেয়ার|প্রদান|দিন|বলুন|লিখে)`,
		`ওটিপি\s*(?:শেয়ার|প্রদান|দিন|বলুন|লিখে)`,
		`পাসওয়ার্ড\s*(?:শেয়ার|প্রদান|দিন|বলুন|লিখে)`,
	}

	hasViolation := false
	for _, pattern := range askPatternsEn {
		re := regexp.MustCompile(pattern)
		if re.MatchString(text) {
			hasViolation = true
			break
		}
	}
	for _, pattern := range askPatternsBn {
		re := regexp.MustCompile(pattern)
		if re.MatchString(text) {
			hasViolation = true
			break
		}
	}

	if hasViolation {
		// Rewrite to a completely safe response instead of letting the violation pass
		if lang == "bn" {
			return "আমরা আপনার পিন, ওটিপি বা পাসওয়ার্ড জানতে চাই না। অনুগ্রহ করে এগুলো কারো সাথে শেয়ার করবেন না। আমাদের টিম আপনার সমস্যাটি অফিসিয়াল চ্যানেলে যাচাই করছে।"
		}
		return "We will never ask for your PIN, OTP, or password. Please do not share these with anyone. Our team is investigating your concern through official support channels."
	}

	return text
}

// Replaces instructions to contact unofficial third parties with official channel guidelines
func sanitizeThirdPartyContacts(text string) string {
	// Look for unofficial links or suspicious phone numbers.
	// If they guide to a suspicious whatsapp link or unofficial number, strip it.
	// (Standard implementation: check for external URLs other than official domain if any)
	// For this challenge, we just ensure that we don't mention any third-party links or numbers.
	suspiciousRegex := regexp.MustCompile(`(?i)\b(?:whatsapp|fb\.com|facebook|telegram|t\.me|bit\.ly|tinyurl)\S+`)
	return suspiciousRegex.ReplaceAllString(text, "official support channels")
}

// Ensures the mandatory safety disclaimer is present in the customer reply
func ensureSafetyDisclaimer(resp *TicketResponse, lang string) {
	disclaimerEn := "Please do not share your PIN or OTP with anyone."
	disclaimerBn := "অনুগ্রহ করে কারো সাথে আপনার পিন বা ওটিপি শেয়ার করবেন না।"

	replyLower := strings.ToLower(resp.CustomerReply)

	if lang == "bn" {
		// Check if it already contains the Bangla disclaimer keywords
		if !strings.Contains(resp.CustomerReply, "পিন") || !strings.Contains(resp.CustomerReply, "ওটিপি") || !strings.Contains(resp.CustomerReply, "শেয়ার") {
			resp.CustomerReply = strings.TrimSpace(resp.CustomerReply)
			if !strings.HasSuffix(resp.CustomerReply, ".") && !strings.HasSuffix(resp.CustomerReply, "।") {
				resp.CustomerReply += "।"
			}
			resp.CustomerReply += " " + disclaimerBn
		}
	} else {
		// English or mixed
		if !strings.Contains(replyLower, "pin") || !strings.Contains(replyLower, "otp") || !strings.Contains(replyLower, "share") {
			resp.CustomerReply = strings.TrimSpace(resp.CustomerReply)
			if !strings.HasSuffix(resp.CustomerReply, ".") && !strings.HasSuffix(resp.CustomerReply, "!") {
				resp.CustomerReply += "."
			}
			resp.CustomerReply += " " + disclaimerEn
		}
	}
}
