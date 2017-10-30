package main

import "regexp"
import "strings"

var /* const */ versionRegexp = regexp.MustCompile("^v[0-9].*")

// RemoveDisambiguationInfixes removes (from pod/container names) version strings and hashes inserted replication controllers and the like.
func RemoveDisambiguationInfixes(podAndContainer string) string {
	split := strings.SplitN(podAndContainer, "/", 2)
	if len(split) < 2 {
		return podAndContainer
	}
	pod, container := split[0], split[1]
	pieces := strings.Split(pod, "-")
	var last string
	for i, piece := range pieces {
		if looksLikeHash(piece) || versionRegexp.MatchString(piece) {
			break
		}
		last = strings.Join(pieces[:i+1], "-")
	}
	return strings.Join([]string{last, container}, "/")
}

// looksLikeHash returns true if piece seems to be one of those pseudo-random disambiguation strings
func looksLikeHash(piece string) bool {
	return len(piece) >= 4 && !strings.ContainsAny(piece, "eyuioa")
}
