package upgrade

// Stub file — full implementation in Task 2.
// These functions are declared here so gate.go compiles.

func RunGateDeploy(opts GateOptions) GateResult {
	return GateResult{Gate: "deployment", Number: 1, Status: "fail", Message: "not implemented"}
}

func RunGateSync(opts GateOptions) GateResult {
	return GateResult{Gate: "sync", Number: 2, Status: "fail", Message: "not implemented"}
}

func RunGateRequest(relayURL string) GateResult {
	return GateResult{Gate: "request-path", Number: 3, Status: "fail", Message: "not implemented"}
}

func RunGateAudit(relayURL string) GateResult {
	return GateResult{Gate: "auditability", Number: 4, Status: "fail", Message: "not implemented"}
}

func RunGatePatch(repoRoot string) GateResult {
	return GateResult{Gate: "patch-intent", Number: 5, Status: "fail", Message: "not implemented"}
}
