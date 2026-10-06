package source

import (
	"testing"
)

func TestSourceFileOffsets(t *testing.T) {
	content := "hello world\nשלום עולם\n👋 🌍 test\n"
	file := NewFile("test.cherri", "file:///test.cherri", 1, content)

	if file.LineCount() != 4 {
		t.Fatalf("expected 4 lines, got %d", file.LineCount())
	}

	// Line 1: "hello world\n" (12 bytes, 12 utf-16)
	p0 := file.PositionFromByteOffset(0)
	if p0.Line != 1 || p0.Column != 1 || p0.UTF16Offset != 0 {
		t.Errorf("expected 1:1 utf16 0, got %v", p0)
	}

	// Line 2: "שלום עולם\n"
	// "hello world\n" is 12 bytes
	pLine2 := file.PositionFromByteOffset(12)
	if pLine2.Line != 2 || pLine2.Column != 1 {
		t.Errorf("expected 2:1 at byte 12, got %v", pLine2)
	}

	// Line 3 has emojis: 👋 (U+1F44B, surrogate pair in UTF-16 = 2 code units, 4 bytes in UTF-8)
	// Let's verify emoji UTF-16 offset handling
	byteStartLine3 := file.lines[2].ByteStart
	pLine3 := file.PositionFromByteOffset(byteStartLine3)
	if pLine3.Line != 3 || pLine3.Column != 1 {
		t.Errorf("expected 3:1 at line 3 start, got %v", pLine3)
	}

	// Position after 👋 (4 bytes)
	pAfterWave := file.PositionFromByteOffset(byteStartLine3 + 4)
	if pAfterWave.Line != 3 || pAfterWave.Column != 2 {
		t.Errorf("expected column 2 after emoji, got %v", pAfterWave)
	}
	if pAfterWave.UTF16Offset-pLine3.UTF16Offset != 2 {
		t.Errorf("expected 2 UTF-16 units for surrogate pair emoji, got diff %d", pAfterWave.UTF16Offset-pLine3.UTF16Offset)
	}

	// Test LSP conversions
	lspPos := file.PositionFromLSP(2, 2) // Line 3 (0-based 2), character 2 (after 👋)
	if lspPos.ByteOffset != byteStartLine3+4 {
		t.Errorf("expected byte offset %d, got %d", byteStartLine3+4, lspPos.ByteOffset)
	}
}
