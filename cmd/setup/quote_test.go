package main

import "testing"

func TestPsQuoteEscapesSingleQuotes(t *testing.T) {
	got := psQuote(`C:\Users\O'Hara\Skiff`)
	want := `'C:\Users\O''Hara\Skiff'`
	if got != want {
		t.Fatalf("引用结果 = %s，希望 %s", got, want)
	}
}
