# QueueStorm

QueueStorm is a ticket-analysis project with two production-style implementations:

- a **Go + Echo** service for fast, concurrent processing
- a **Python + FastAPI** service for quick deployment and API-first iteration

Both versions analyze customer complaints together with transaction history, determine the most relevant transaction, route the case to the right department, and return a structured JSON response with safety guardrails.

## Live Deployments

- **FastAPI**: [https://queuestorm-r6uu.onrender.com/](https://queuestorm-r6uu.onrender.com/)
- **Echo**: [https://queuestorm-golang.onrender.com/](https://queuestorm-golang.onrender.com/)

## Project Overview

The repo contains two separate app folders:

```text
QueueStorm/
├─ queuestorm-echo/       # Go + Echo implementation
└─ QueueStorm_FastApi/    # Python + FastAPI implementation
```

Each implementation follows the same overall pipeline:

1. **API layer** validates the incoming request.
2. **Reasoning layer** analyzes the complaint and transaction history.
3. **LLM/API call** generates the natural-language explanation.
4. **Safety filter** rewrites risky output.
5. **JSON response** is returned to the judge or client.

## Architecture Comparison

```mermaid
flowchart TB
    subgraph E["Echo version"]
        E1[Go + Echo]
        E2[Request validation and sanitization]
        E3[Internal worker queue]
        E4[Tier 1 deterministic rule engine]
        E5[Tier 2a Gemini primary]
        E6[Tier 2b Gemini backup]
        E7[Tier 2c Groq backup]
        E8[Safety filter and JSON response]
        E1 --> E2 --> E3 --> E4 --> E5 --> E6 --> E7 --> E8
    end

    subgraph F["FastAPI version"]
        F1[Python + FastAPI]
        F2[Request validation with Pydantic]
        F3[Reasoning engine]
        F4[AI API call]
        F5[Safety filter]
        F1 --> F2 --> F3 --> F4 --> F5
    end
```

| Area | Echo | FastAPI |
| --- | --- | --- |
| Language | Go | Python |
| Framework | Echo | FastAPI |
| Best for | Maximum concurrency and a compact service | Fast iteration and simple API hosting |
| Request validation | Strong typed structs | Pydantic models |
| AI integration | Gemini primary, Gemini backup, Groq fallback | External LLM provider calls |
| Safety handling | Output rewriting / guardrails | Output rewriting / guardrails |

## Which One Should You Use?

- Use **Echo** if you want the fastest runtime path and a Go-based service.
- Use **FastAPI** if you want a simpler Python deployment workflow.
- Keep both if you want a fallback option or want to compare behavior side by side.

## Repository Contents

- `queuestorm-echo/` — Go service, Dockerfile, test runner, and deployment-ready Echo app
- `QueueStorm_FastApi/` — FastAPI service with schema validation and deployment config

## Local Run

### Echo

```bash
cd queuestorm-echo
go run cmd/server/main.go
```

### FastAPI

```bash
cd QueueStorm_FastApi
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
uvicorn app.main:app --reload
```

## Safety Notes

Both implementations are designed to:

- avoid unsafe credential requests
- avoid direct refund promises
- keep users on official support channels
- return clean JSON for downstream automation

## Deployment Notes

- The live deployment links are listed above for quick testing.
- Each service can be deployed independently.
- Update environment variables in your hosting platform before going live.
