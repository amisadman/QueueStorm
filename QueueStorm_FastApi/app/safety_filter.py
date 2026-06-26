import re
from typing import Any, Dict, Iterable


SAFE_CREDENTIAL_REPLY = (
	"Thank you for contacting us. We have received your request and will review it through official support channels."
)
SAFE_OFFICIAL_CHANNELS_ONLY = "Please contact official support channels only."
SAFE_REFUND_LANGUAGE = "Any eligible amount will be returned through official channels."


_CREDENTIAL_PATTERNS: Iterable[re.Pattern[str]] = (
	re.compile(r"\b(pin|otp|password)\b", re.IGNORECASE),
	re.compile(r"\b(full\s+card\s+number|card\s+number|card\s+details|cvv|cvc)\b", re.IGNORECASE),
	re.compile(r"\bverification\b", re.IGNORECASE),
	re.compile(r"\bverify\b", re.IGNORECASE),
	re.compile(r"\bauthentication\b", re.IGNORECASE),
)

_REFUND_PATTERNS: Iterable[re.Pattern[str]] = (
	re.compile(r"\bwe\s+will\s+refund\s+you\b", re.IGNORECASE),
	re.compile(r"\byour\s+money\s+has\s+been\s+returned\b", re.IGNORECASE),
	re.compile(r"\bhas\s+been\s+refunded\b", re.IGNORECASE),
	re.compile(r"\brefund\s+has\s+been\s+processed\b", re.IGNORECASE),
	re.compile(r"\bamount\s+has\s+been\s+returned\b", re.IGNORECASE),
	re.compile(r"\bcredited\s+back\b", re.IGNORECASE),
	re.compile(r"\breversal\s+(has\s+been\s+)?(completed|done)\b", re.IGNORECASE),
	re.compile(r"\breturned\s+to\s+your\s+account\b", re.IGNORECASE),
)

_THIRD_PARTY_PATTERNS: Iterable[re.Pattern[str]] = (
	re.compile(r"\bcontact\b.*\b(merchant|third\s+party|bank|agent|seller|person|private|personal|whatsapp|telegram|branch|number|outside\s+official|unofficial)\b", re.IGNORECASE),
	re.compile(r"\bcall\b.*\b(merchant|bank|agent|seller|person|private|personal|whatsapp|telegram|branch|number)\b", re.IGNORECASE),
	re.compile(r"\boutside\s+official\s+channels\b", re.IGNORECASE),
	re.compile(r"\bunofficial\s+channel(s)?\b", re.IGNORECASE),
)


def _matches_any(text: Any, patterns: Iterable[re.Pattern[str]]) -> bool:
	if not isinstance(text, str):
		return False
	return any(pattern.search(text) for pattern in patterns)


def _safe_text(value: Any, fallback: str) -> str:
	if isinstance(value, str) and value.strip():
		return value
	return fallback


def check_and_fix_reply(response: Dict[str, Any]) -> Dict[str, Any]:
	updated_response = dict(response)
	safety_violation_fixed = False

	customer_reply = updated_response.get("customer_reply")
	recommended_next_action = updated_response.get("recommended_next_action")

	credential_violation = _matches_any(customer_reply, _CREDENTIAL_PATTERNS) or _matches_any(recommended_next_action, _CREDENTIAL_PATTERNS)
	refund_violation = _matches_any(customer_reply, _REFUND_PATTERNS) or _matches_any(recommended_next_action, _REFUND_PATTERNS)
	third_party_violation = _matches_any(customer_reply, _THIRD_PARTY_PATTERNS) or _matches_any(recommended_next_action, _THIRD_PARTY_PATTERNS)

	if credential_violation:
		updated_response["customer_reply"] = SAFE_CREDENTIAL_REPLY
		updated_response["recommended_next_action"] = SAFE_OFFICIAL_CHANNELS_ONLY
		safety_violation_fixed = True
	elif refund_violation:
		updated_response["customer_reply"] = SAFE_REFUND_LANGUAGE
		updated_response["recommended_next_action"] = SAFE_REFUND_LANGUAGE
		safety_violation_fixed = True
	elif third_party_violation:
		updated_response["customer_reply"] = SAFE_OFFICIAL_CHANNELS_ONLY
		updated_response["recommended_next_action"] = SAFE_OFFICIAL_CHANNELS_ONLY
		safety_violation_fixed = True

	updated_response["customer_reply"] = _safe_text(updated_response.get("customer_reply"), SAFE_CREDENTIAL_REPLY)
	updated_response["recommended_next_action"] = _safe_text(updated_response.get("recommended_next_action"), SAFE_OFFICIAL_CHANNELS_ONLY)
	updated_response["safety_violation_fixed"] = safety_violation_fixed
	return updated_response