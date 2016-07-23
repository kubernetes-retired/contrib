package mungerutil

import "github.com/google/go-github/github"

func HasValidReviwer(reviewers []*github.User) bool {
	for _, r := range reviewers {
		if r != nil && r.Login != nil {
			return true
		}
	}
	return false
}
