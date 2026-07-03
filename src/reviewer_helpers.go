package main

import "github.com/benbjohnson/immutable"

type Reviewer[TIssue any] interface {
	Approved() bool
	ShouldReset() bool
	Issues() *immutable.List[TIssue]
}

func reviewOk[TSeverity ~string, TIssue any, R Reviewer[TIssue]](r R) bool {
	return r.Approved() && !checkIssuesTyped[TSeverity, TIssue](r.Issues())
}

func shouldReset[TIssue any, R Reviewer[TIssue]](r R) bool {
	return r.ShouldReset()
}

func checkIssuesTyped[TSeverity ~string, TIssue any](list *immutable.List[TIssue]) bool {
	if list == nil {
		return false
	}
	itr := list.Iterator()
	itr.First()
	for !itr.Done() {
		_, v := itr.Next()
		i, ok := any(&v).(interface{ Severity() TSeverity })
		if !ok {
			panic("issue does not satisfy Severity() constraint")
		}
		if i.Severity() == "high" {
			return true
		}
	}
	return false
}
