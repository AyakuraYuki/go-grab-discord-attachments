package main

import (
	"regexp"
	"testing"
)

func TestCut(t *testing.T) {
	re := regexp.MustCompile(`\.[a-zA-Z0-9]+`)
	t.Log(re.FindString(`abc.mp4&cv3=98627607869875abfe59865aeb5978659ae5b69`))
}
