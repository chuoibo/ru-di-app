package repo

import "context"

// ADR-0056 §2.4.3: an applied amendment replaces a batch version's
// obligations, and the receipts and payment reports on a replaced obligation
// count toward the one of the same pair that succeeds it. The table that
// records successions belongs to internal/dieuchinh, which only a deployment
// serving amendments migrates; there, its front door marks every request with
// WithSuccessions and the reads below follow the chain. Unmarked, each read
// is the Python statement byte for byte, which is what parity compares.

type successionsKey struct{}

// WithSuccessions marks ctx as served where amendments are on.
func WithSuccessions(ctx context.Context) context.Context {
	return context.WithValue(ctx, successionsKey{}, true)
}

func successionsOn(ctx context.Context) bool {
	on, _ := ctx.Value(successionsKey{}).(bool)
	return on
}

// obligationChain is a recursive CTE, chain(id): the obligation $1 and every
// obligation it succeeded, transitively.
const obligationChain = `WITH RECURSIVE chain(id) AS (
	SELECT $1::UUID
	UNION
	SELECT collection_obligation_successions.old_obligation_id
	  FROM collection_obligation_successions JOIN chain ON collection_obligation_successions.new_obligation_id = chain.id)
`
