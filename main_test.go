package main

import (
	"regexp"
	"testing"
)

func TestCut(t *testing.T) {
	re := regexp.MustCompile(`\.[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*`)
	t.Log(re.FindString(`abc.mp4&cv3=98627607869875abfe59865aeb5978659ae5b69`))
	t.Log(re.FindString(`foo.tar.gz?t=123&cv0=3221967659ac754676b6fd0789e7064e`))
	t.Log(re.FindString(``))
}
