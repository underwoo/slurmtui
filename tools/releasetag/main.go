package main

import (
	"fmt"
	"os"
	"regexp"
)

var releaseTagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-alpha\.(0|[1-9][0-9]*))?$`)

func releaseKind(tag string) (string, error) {
	match := releaseTagPattern.FindStringSubmatch(tag)
	if match == nil {
		return "", fmt.Errorf("tag must be vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-alpha.N: %s", tag)
	}
	if match[4] != "" {
		return "prerelease", nil
	}
	return "stable", nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: releasetag TAG")
		os.Exit(2)
	}
	kind, err := releaseKind(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(kind)
}
