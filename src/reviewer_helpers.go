package main

type Reviewer[TIssue any] interface {
	Approved() bool
	ShouldReset() bool
	Issues() []TIssue
}

func reviewOk[TSeverity ~string, TIssue any, R Reviewer[TIssue]](r R) bool {
	return r.Approved() && !checkIssuesTyped[TSeverity](r.Issues())
}

func shouldReset[TIssue any, R Reviewer[TIssue]](r R) bool {
	return r.ShouldReset()
}

func checkIssuesTyped[TSeverity ~string, TIssue any](slice []TIssue) bool {
	for i := range slice {
		iAny := &slice[i]
		i, ok := any(iAny).(interface{ Severity() TSeverity })
		if !ok {
			panic("issue does not satisfy Severity() constraint")
		}
		if i.Severity() == "high" {
			return true
		}
	}
	return false
}
