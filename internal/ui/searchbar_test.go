package ui

import "testing"

func TestByteToRuneCol(t *testing.T) {
	// "│ " is a 3-byte box-drawing rune + space; "ab" follows.
	line := "│ ab"
	// byte offset of "a" is 3 (3-byte rune) + 1 (space) = 4; rune col is 2.
	if got := byteToRuneCol(line, 4); got != 2 {
		t.Fatalf("byteToRuneCol(%q,4) = %d, want 2", line, got)
	}
	// Pure ASCII: byte offset == rune col.
	if got := byteToRuneCol("hello", 3); got != 3 {
		t.Fatalf("ascii byteToRuneCol = %d, want 3", got)
	}
	// Out-of-range byte offset clamps to len.
	if got := byteToRuneCol("ab", 99); got != 2 {
		t.Fatalf("clamp byteToRuneCol = %d, want 2", got)
	}
}
