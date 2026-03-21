package sync

// ActionType represents the kind of reconciliation action.
type ActionType string

const (
	ActionCreate   ActionType = "create"
	ActionUpdate   ActionType = "update"
	ActionDelete   ActionType = "delete"
	ActionNoChange ActionType = "no-change"
)

// Action describes a single reconciliation step to move from live to desired state.
type Action struct {
	Type         ActionType // create, update, delete, no-change
	ResourceType string     // "channel", "provider", "routing-rule", "config", "pricing", "cookie"
	ResourceID   string     // the match key (channel name, provider id, rule id, model name, cookie prefix)
	Desired      any        // typed desired-state struct (nil for delete)
	Live         any        // typed live-state struct (nil for create)
	Diff         string     // human-readable diff for updates
}

// ResultStatus describes the outcome of applying an action.
type ResultStatus string

const (
	StatusOK               ResultStatus = "ok"
	StatusAppliedWithDrift ResultStatus = "applied-with-drift"
	StatusUnverified       ResultStatus = "unverified"
	StatusFailed           ResultStatus = "failed"
	StatusSkipped          ResultStatus = "skipped"
)

// Result captures the outcome of applying a single Action.
type Result struct {
	Action   Action       // the action that was applied
	Status   ResultStatus // outcome status
	Error    error        // non-nil if StatusFailed
	ReadBack any          // what we read back after write (for verify step)
	DriftMsg string       // if applied-with-drift, describes the difference
}

// SyncReport aggregates results from a full reconciliation run.
type SyncReport struct {
	Results       []Result // individual results
	TotalActions  int      // total actions planned
	Succeeded     int      // actions that completed with StatusOK
	Failed        int      // actions that completed with StatusFailed
	DriftWarnings int      // actions that completed with StatusAppliedWithDrift
	Skipped       int      // actions that were skipped
}

// Reconciler defines the Plan/Apply/Verify contract for reconciling a resource type.
// Every reconciler (channel, provider, routing-rule, config, pricing, cookie) implements this.
type Reconciler interface {
	// Name returns the resource type name (e.g. "channel", "provider")
	Name() string
	// Plan compares desired vs live state and returns actions to reconcile
	Plan(desired, live any) ([]Action, error)
	// Apply executes a single action (create/update/delete)
	Apply(action Action) (*Result, error)
	// Verify reads back the resource after Apply and checks it matches expected state
	Verify(action Action, result *Result) error
}
