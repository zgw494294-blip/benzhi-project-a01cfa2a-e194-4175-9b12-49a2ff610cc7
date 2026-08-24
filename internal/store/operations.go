package store

func operationForAction(action string) string {
	switch action {
	case "case.created":
		return "create_case"
	case "revision.submitted":
		return "submit_revision"
	case "check.completed":
		return "run_check"
	case "finding.added":
		return "add_finding"
	case "review.conclusions_set":
		return "set_conclusions"
	case "retake.requested":
		return "request_retake"
	case "finding.closed":
		return "close_finding"
	case "case.frozen":
		return "freeze"
	case "credential.issued":
		return "issue"
	default:
		return action
	}
}
