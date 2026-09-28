package llm

// The per-turn budgets beside MaxModelCallsPerTurn (llm.go), in one place so
// the engine enforces and the eval budgets from the same numbers
// (docs/architecture/03-ai-engine-hop-dong.md, «Luật không heuristic»).
// Enforcing a budget is deterministic bookkeeping, never a judgement about
// what the person meant.
const (
	// MaxToolCallsPerTurn bounds the tool calls of one turn, across every
	// agent step; the call past it is refused (tools.SoCai.Giu).
	MaxToolCallsPerTurn = 10
	// MaxToolCallsNep is Nếp's tighter ceiling within MaxToolCallsPerTurn:
	// three steps rarely need more, and a smaller ceiling bounds latency
	// (research agentic-rag-tools §0.3).
	MaxToolCallsNep = 6
	// MaxStepsNhom is the group assistant's agent steps (model calls inside
	// the loop); the last one runs with function calling off.
	MaxStepsNhom = 4
	// MaxStepsNep is Nếp's agent steps.
	MaxStepsNep = 3
	// MaxCorrectiveRounds is how many corrective retrievals (a CRAG round:
	// relax a soft constraint or rewrite the query) one turn may run.
	MaxCorrectiveRounds = 1
	// MaxEmbedCallsPerTurn bounds the embedding calls of one turn (the
	// query side of retrieval and of memory recall), counted apart from
	// model calls.
	MaxEmbedCallsPerTurn = 2
	// MaxRerankCallsPerTurn bounds the reranker calls of one turn (the first
	// retrieval and one corrective round), counted apart from model calls
	// (research reflection-verification §5.5).
	MaxRerankCallsPerTurn = 2
)
