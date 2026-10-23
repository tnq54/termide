package editor

import "strings"

type Match struct { Line, Column int; Text string }
func Search(text, query string) []Match {
	if query == "" { return nil }
	var matches []Match
	for lineNo, line := range strings.Split(text, "\n") {
		for start := 0; ; {
			index := strings.Index(line[start:], query)
			if index < 0 { break }
			index += start
			matches = append(matches, Match{Line: lineNo + 1, Column: index + 1, Text: line[index:index+len(query)]})
			start = index + len(query)
			if start > len(line) { break }
		}
	}
	return matches
}
