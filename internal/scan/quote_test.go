package scan

import "testing"

func TestRmRfQuotesSpaces(t *testing.T) {
	got := rmRf("/tmp/foo bar")
	if got != "rm -rf '/tmp/foo bar'" {
		t.Fatal(got)
	}
	plain := rmRf("/private/tmp/kensi-app")
	if plain != "rm -rf /private/tmp/kensi-app" {
		t.Fatal(plain)
	}
}
