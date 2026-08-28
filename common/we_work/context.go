package we_work

import "context"

type corpContextKey struct{}

// WithCorpID carries the verified enterprise through nested calls and jobs.
// Never fall back to process-wide credentials when the enterprise is missing.
func WithCorpID(ctx context.Context, corpID string) context.Context {
	return context.WithValue(ctx, corpContextKey{}, corpID)
}

func CorpID(ctx context.Context) string {
	corpID, _ := ctx.Value(corpContextKey{}).(string)
	return corpID
}
