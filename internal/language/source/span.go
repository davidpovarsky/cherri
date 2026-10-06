package source

import (
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

// DocumentID uniquely identifies a source document within a compilation session or LSP server.
type DocumentID string

// Position represents a position in a source document.
// Line and Column are 1-based for human/CLI display, but 0-based for LSP.
// ByteOffset is 0-based byte index in UTF-8.
// UTF16Offset is 0-based code unit offset in UTF-16 (used for LSP and Apple attachment ranges).
type Position struct {
	Line        int `json:"line"`        // 1-based line number
	Column      int `json:"column"`      // 1-based column (in UTF-8 runes)
	ByteOffset  int `json:"byteOffset"`  // 0-based UTF-8 byte offset
	UTF16Offset int `json:"utf16Offset"` // 0-based UTF-16 code unit offset
}

// LSPLine returns 0-based line number for LSP protocol.
func (p Position) LSPLine() int {
	if p.Line <= 0 {
		return 0
	}
	return p.Line - 1
}

// LSPCharacter returns 0-based UTF-16 character offset for LSP protocol on this line.
func (p Position) LSPCharacter(lineStartUTF16 int) int {
	c := p.UTF16Offset - lineStartUTF16
	if c < 0 {
		return 0
	}
	return c
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Span represents a byte and character range in a specific document.
type Span struct {
	Document DocumentID `json:"document"`
	Start    Position   `json:"start"`
	End      Position   `json:"end"`
}

func (s Span) String() string {
	if s.Document == "" {
		return fmt.Sprintf("%s-%s", s.Start, s.End)
	}
	return fmt.Sprintf("%s:%s-%s", s.Document, s.Start, s.End)
}

// IsZero returns true if span is empty/uninitialized.
func (s Span) IsZero() bool {
	return s.Start.ByteOffset == 0 && s.End.ByteOffset == 0 && s.Start.Line == 0 && s.End.Line == 0
}

// Contains returns true if the given byte offset is within this span.
func (s Span) Contains(byteOffset int) bool {
	return byteOffset >= s.Start.ByteOffset && byteOffset <= s.End.ByteOffset
}

// Merge returns the minimal span containing both s and other.
func (s Span) Merge(other Span) Span {
	if s.IsZero() {
		return other
	}
	if other.IsZero() {
		return s
	}
	doc := s.Document
	if doc == "" {
		doc = other.Document
	}
	start := s.Start
	if other.Start.ByteOffset < start.ByteOffset {
		start = other.Start
	}
	end := s.End
	if other.End.ByteOffset > end.ByteOffset {
		end = other.End
	}
	return Span{
		Document: doc,
		Start:    start,
		End:      end,
	}
}

// TriviaKind represents trivia attached to tokens (comments, whitespace).
type TriviaKind int

const (
	TriviaWhitespace TriviaKind = iota
	TriviaLineComment
	TriviaBlockComment
)

// Trivia represents whitespace or comments preserved for formatting and AST fidelity.
type Trivia struct {
	Kind TriviaKind
	Text string
	Span Span
}

// LineInfo tracks index mapping for a line in a SourceFile.
type LineInfo struct {
	LineNumber  int // 1-based
	ByteStart   int // 0-based UTF-8 byte index
	ByteEnd     int // 0-based UTF-8 byte index (before newline)
	UTF16Start  int // 0-based UTF-16 code unit index
}

// File represents a source file in memory with full bidirectional offset mappings.
type File struct {
	ID      DocumentID
	URI     string
	Version int
	Content string
	lines   []LineInfo
}

// NewFile constructs a new SourceFile and pre-computes line and UTF-16 offset tables.
func NewFile(id DocumentID, uri string, version int, content string) *File {
	f := &File{
		ID:      id,
		URI:     uri,
		Version: version,
		Content: content,
	}
	f.indexLines()
	return f
}

func (f *File) indexLines() {
	f.lines = nil
	byteOffset := 0
	utf16Offset := 0
	lineNum := 1

	lineByteStart := 0
	lineUTF16Start := 0

	s := f.Content
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]

		rUTF16Len := 1
		if r > 0xFFFF {
			rUTF16Len = 2
		}

		if r == '\n' {
			f.lines = append(f.lines, LineInfo{
				LineNumber: lineNum,
				ByteStart:  lineByteStart,
				ByteEnd:    byteOffset,
				UTF16Start: lineUTF16Start,
			})
			lineNum++
			byteOffset += size
			utf16Offset += rUTF16Len
			lineByteStart = byteOffset
			lineUTF16Start = utf16Offset
		} else {
			byteOffset += size
			utf16Offset += rUTF16Len
		}
	}

	// Trailing line
	f.lines = append(f.lines, LineInfo{
		LineNumber: lineNum,
		ByteStart:  lineByteStart,
		ByteEnd:    byteOffset,
		UTF16Start: lineUTF16Start,
	})
}

// PositionFromByteOffset converts a 0-based UTF-8 byte offset to a full Position.
func (f *File) PositionFromByteOffset(offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(f.Content) {
		offset = len(f.Content)
	}

	// Binary search for line
	low, high := 0, len(f.lines)-1
	lineIdx := 0
	for low <= high {
		mid := (low + high) / 2
		if f.lines[mid].ByteStart <= offset {
			lineIdx = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	lineInfo := f.lines[lineIdx]
	col := 1
	utf16Off := lineInfo.UTF16Start

	// Scan from lineByteStart to offset
	sub := f.Content[lineInfo.ByteStart:offset]
	for len(sub) > 0 {
		r, size := utf8.DecodeRuneInString(sub)
		sub = sub[size:]
		col++
		if r > 0xFFFF {
			utf16Off += 2
		} else {
			utf16Off++
		}
	}

	return Position{
		Line:        lineInfo.LineNumber,
		Column:      col,
		ByteOffset:  offset,
		UTF16Offset: utf16Off,
	}
}

// PositionFromLSP converts 0-based LSP line and UTF-16 character to Position.
func (f *File) PositionFromLSP(line int, character int) Position {
	if line < 0 {
		line = 0
	}
	if line >= len(f.lines) {
		line = len(f.lines) - 1
	}

	lineInfo := f.lines[line]
	byteOffset := lineInfo.ByteStart
	col := 1
	currUTF16 := 0

	lineContent := f.Content[lineInfo.ByteStart:]
	if line < len(f.lines)-1 {
		lineContent = f.Content[lineInfo.ByteStart:f.lines[line+1].ByteStart]
	}

	for len(lineContent) > 0 && currUTF16 < character {
		r, size := utf8.DecodeRuneInString(lineContent)
		if r == '\n' {
			break
		}
		lineContent = lineContent[size:]
		byteOffset += size
		col++
		if r > 0xFFFF {
			currUTF16 += 2
		} else {
			currUTF16++
		}
	}

	return Position{
		Line:        lineInfo.LineNumber,
		Column:      col,
		ByteOffset:  byteOffset,
		UTF16Offset: lineInfo.UTF16Start + currUTF16,
	}
}

// SpanForOffsets creates a Span between two byte offsets.
func (f *File) SpanForOffsets(startOffset, endOffset int) Span {
	return Span{
		Document: f.ID,
		Start:    f.PositionFromByteOffset(startOffset),
		End:      f.PositionFromByteOffset(endOffset),
	}
}

// Slice returns the substring corresponding to the span.
func (f *File) Slice(span Span) string {
	start := span.Start.ByteOffset
	end := span.End.ByteOffset
	if start < 0 {
		start = 0
	}
	if end > len(f.Content) {
		end = len(f.Content)
	}
	if start >= end {
		return ""
	}
	return f.Content[start:end]
}

// UTF16Length returns the total UTF-16 code unit length of the file content.
func (f *File) UTF16Length() int {
	return len(utf16.Encode([]rune(f.Content)))
}

// LineCount returns total number of lines.
func (f *File) LineCount() int {
	return len(f.lines)
}

// LineStartUTF16 returns the UTF-16 code unit offset at the start of the given 1-based line.
func (f *File) LineStartUTF16(line int) int {
	if line < 1 || line > len(f.lines) {
		return 0
	}
	return f.lines[line-1].UTF16Start
}
