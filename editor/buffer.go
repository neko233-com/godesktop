// Package editor provides versioned UTF-8 documents with VS Code/LSP UTF-16
// positions. Buffers belong to one UI thread; Snapshot is safe to hand to workers.
package editor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Selection struct {
	Anchor Position `json:"anchor"`
	Active Position `json:"active"`
}

func (s Selection) Range() Range {
	if less(s.Active, s.Anchor) {
		return Range{s.Active, s.Anchor}
	}
	return Range{s.Anchor, s.Active}
}

type Edit struct {
	Range Range  `json:"range"`
	Text  string `json:"newText"`
}
type Change struct {
	Range       Range  `json:"range"`
	RangeLength int    `json:"rangeLength"`
	Text        string `json:"text"`
}
type ChangeEvent struct {
	Version int      `json:"version"`
	Changes []Change `json:"contentChanges"`
}
type Snapshot struct {
	Version   int
	EOL       string
	Selection Selection
	lines     []string
	identity  *bufferIdentity
}

func (s Snapshot) Text() string   { return strings.Join(s.lines, s.EOL) }
func (s Snapshot) LineCount() int { return len(s.lines) }
func (s Snapshot) Line(index int) string {
	if index < 0 || index >= len(s.lines) {
		return ""
	}
	return s.lines[index]
}

type history struct {
	start                         Position
	removed, inserted             string
	before, after                 Selection
	beforeRevision, afterRevision uint64
	beforeEOL, afterEOL           string
}
type Buffer struct {
	identity                              *bufferIdentity
	lines                                 []string
	eol                                   string
	version                               int
	selection                             Selection
	revision, nextRevision, savedRevision uint64
	undo, redo                            []history
	historyBytes                          int
}

const MaxHistoryBytes = 16 << 20
const MaxHistoryEntries = 2048

type bufferIdentity struct{ token byte }

func New(text string) (*Buffer, error) {
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return nil, errors.New("document must contain valid UTF-8 text without NUL")
	}
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	return &Buffer{identity: &bufferIdentity{}, lines: strings.Split(normalize(text), "\n"), eol: eol, version: 1}, nil
}
func normalize(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}
func less(a, b Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
}
func units(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r > 0xffff {
			n++
		}
	}
	return n
}
func byteColumn(s string, column int) (int, error) {
	if column < 0 {
		return 0, errors.New("negative UTF-16 column")
	}
	n := 0
	for index, r := range s {
		if n == column {
			return index, nil
		}
		n++
		if r > 0xffff {
			n++
		}
		if n > column {
			return 0, errors.New("position splits a UTF-16 surrogate pair")
		}
	}
	if n == column {
		return len(s), nil
	}
	return 0, errors.New("position exceeds line")
}
func (b *Buffer) validate(p Position) (int, error) {
	if p.Line < 0 || p.Line >= len(b.lines) {
		return 0, fmt.Errorf("invalid line %d", p.Line)
	}
	return byteColumn(b.lines[p.Line], p.Character)
}
func (b *Buffer) Version() int   { return b.version }
func (b *Buffer) LineCount() int { return len(b.lines) }
func (b *Buffer) Line(i int) string {
	if i < 0 || i >= len(b.lines) {
		return ""
	}
	return b.lines[i]
}
func (b *Buffer) EOL() string          { return b.eol }
func (b *Buffer) Text() string         { return strings.Join(b.lines, b.eol) }
func (b *Buffer) Dirty() bool          { return b.revision != b.savedRevision }
func (b *Buffer) MarkSaved()           { b.savedRevision = b.revision }
func (b *Buffer) Selection() Selection { return b.selection }
func (b *Buffer) Snapshot() Snapshot {
	return Snapshot{b.version, b.eol, b.selection, b.lines, b.identity}
}
func (b *Buffer) SetSelection(s Selection) error {
	if _, err := b.validate(s.Anchor); err != nil {
		return err
	}
	if _, err := b.validate(s.Active); err != nil {
		return err
	}
	b.selection = s
	return nil
}
func (b *Buffer) PositionFromRunes(line, column int) Position {
	line = max(0, min(line, len(b.lines)-1))
	character := 0
	for _, r := range []rune(b.lines[line])[:max(0, min(column, utf8.RuneCountInString(b.lines[line])))] {
		character++
		if r > 0xffff {
			character++
		}
	}
	return Position{line, character}
}
func (b *Buffer) RuneColumn(p Position) (int, error) {
	index, err := b.validate(p)
	if err != nil {
		return 0, err
	}
	return utf8.RuneCountInString(b.lines[p.Line][:index]), nil
}

// OffsetAt uses UTF-16 code units, including the document's configured EOL.
func (b *Buffer) OffsetAt(p Position) (int, error) {
	if _, err := b.validate(p); err != nil {
		return 0, err
	}
	n := p.Character
	for _, line := range b.lines[:p.Line] {
		n += units(line) + len(b.eol)
	}
	return n, nil
}
func (b *Buffer) PositionAt(offset int) Position {
	offset = max(0, offset)
	for i, line := range b.lines {
		length := units(line)
		if offset <= length {
			n := 0
			for _, r := range line {
				step := 1
				if r > 0xffff {
					step = 2
				}
				if n+step > offset {
					break
				}
				n += step
			}
			return Position{i, n}
		}
		if offset < length+len(b.eol) {
			return Position{i, length}
		}
		offset -= length + len(b.eol)
	}
	i := len(b.lines) - 1
	return Position{i, units(b.lines[i])}
}
func (b *Buffer) RangeText(r Range) (string, error) {
	start, err := b.validate(r.Start)
	if err != nil {
		return "", err
	}
	end, err := b.validate(r.End)
	if err != nil {
		return "", err
	}
	if less(r.End, r.Start) {
		return "", errors.New("range is reversed")
	}
	if r.Start.Line == r.End.Line {
		return b.lines[r.Start.Line][start:end], nil
	}
	parts := []string{b.lines[r.Start.Line][start:]}
	parts = append(parts, b.lines[r.Start.Line+1:r.End.Line]...)
	parts = append(parts, b.lines[r.End.Line][:end])
	return strings.Join(parts, b.eol), nil
}
func endOf(start Position, text string) Position {
	parts := strings.Split(normalize(text), "\n")
	if len(parts) == 1 {
		return Position{start.Line, start.Character + units(parts[0])}
	}
	return Position{start.Line + len(parts) - 1, units(parts[len(parts)-1])}
}
func (b *Buffer) splice(r Range, text string) {
	start, _ := b.validate(r.Start)
	end, _ := b.validate(r.End)
	parts := strings.Split(b.lines[r.Start.Line][:start]+normalize(text)+b.lines[r.End.Line][end:], "\n")
	next := make([]string, 0, len(b.lines)-(r.End.Line-r.Start.Line)+len(parts)-1)
	next = append(next, b.lines[:r.Start.Line]...)
	next = append(next, parts...)
	next = append(next, b.lines[r.End.Line+1:]...)
	b.lines = next
}

// Apply validates the whole transaction before mutation. Edits use positions in
// the old document; overlapping ranges are rejected. Returned changes are in
// descending order and can be sent directly as incremental LSP changes.
func (b *Buffer) Apply(edits []Edit, selection *Selection) (ChangeEvent, error) {
	return b.apply(context.Background(), edits, selection, nil)
}

// A cancellable application is private: only a worker-owned candidate may be
// discarded after cancellation. Public Apply never partially cancels a buffer.
func (b *Buffer) apply(ctx context.Context, edits []Edit, selection *Selection, captured *history) (ChangeEvent, error) {
	if err := ctx.Err(); err != nil {
		return ChangeEvent{}, err
	}
	if len(edits) == 0 {
		return ChangeEvent{Version: b.version}, nil
	}
	edits = slices.Clone(edits)
	sort.SliceStable(edits, func(i, j int) bool { return less(edits[i].Range.Start, edits[j].Range.Start) })
	for i, e := range edits {
		if err := ctx.Err(); err != nil {
			return ChangeEvent{}, err
		}
		if !utf8.ValidString(e.Text) || strings.ContainsRune(e.Text, 0) {
			return ChangeEvent{}, errors.New("edit must contain valid UTF-8 text without NUL")
		}
		if _, err := b.RangeText(e.Range); err != nil {
			return ChangeEvent{}, err
		}
		if i > 0 && (less(e.Range.Start, edits[i-1].Range.End) || e.Range.Start == edits[i-1].Range.Start) {
			return ChangeEvent{}, errors.New("overlapping edits")
		}
	}
	start, end := edits[0].Range.Start, edits[len(edits)-1].Range.End
	removed, _ := b.RangeText(Range{start, end})
	before := b.selection
	// Transform selection offsets against the original transaction.
	anchor, _ := b.OffsetAt(before.Anchor)
	active, _ := b.OffsetAt(before.Active)
	transform := func(offset int) int {
		delta := 0
		for _, e := range edits {
			a, _ := b.OffsetAt(e.Range.Start)
			z, _ := b.OffsetAt(e.Range.End)
			inserted := strings.ReplaceAll(normalize(e.Text), "\n", b.eol)
			if offset < a {
				break
			}
			if offset <= z {
				return a + delta + units(inserted)
			}
			delta += units(inserted) - (z - a)
		}
		return offset + delta
	}
	anchor, active = transform(anchor), transform(active)
	changes := make([]Change, 0, len(edits))
	addedUnits := 0
	for i := len(edits) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return ChangeEvent{}, err
		}
		e := edits[i]
		text := strings.ReplaceAll(normalize(e.Text), "\n", b.eol)
		old, _ := b.RangeText(e.Range)
		addedUnits += units(text) - units(old)
		changes = append(changes, Change{e.Range, units(old), text})
		b.splice(e.Range, text)
	}
	after := Selection{b.PositionAt(anchor), b.PositionAt(active)}
	if selection != nil {
		after = *selection
	}
	if _, err := b.validate(after.Anchor); err != nil {
		after.Anchor = b.PositionAt(anchor)
	}
	if _, err := b.validate(after.Active); err != nil {
		after.Active = b.PositionAt(active)
	}
	b.selection = after
	startOffset, _ := b.OffsetAt(start)
	inserted, _ := b.RangeText(Range{start, b.PositionAt(startOffset + units(removed) + addedUnits)})
	b.nextRevision++
	item := history{start, removed, inserted, before, after, b.revision, b.nextRevision, b.eol, b.eol}
	if captured != nil {
		*captured = item
	}
	b.revision = b.nextRevision
	b.record(item)
	b.version++
	return ChangeEvent{b.version, changes}, nil
}

func (b *Buffer) record(item history) {
	b.redo = nil
	cost := len(item.removed) + len(item.inserted)
	if cost > MaxHistoryBytes {
		b.undo = nil
		b.historyBytes = 0
	} else {
		b.undo = append(b.undo, item)
		b.historyBytes += cost
		for len(b.undo) > MaxHistoryEntries || b.historyBytes > MaxHistoryBytes {
			b.historyBytes -= len(b.undo[0].removed) + len(b.undo[0].inserted)
			b.undo[0] = history{}
			b.undo = b.undo[1:]
		}
	}
}

// Reload adopts a disk snapshot as the saved revision without replacing the
// buffer identity or resetting its protocol version. It is one bounded undo
// transaction, including EOL changes; undo restores the prior unsaved state and
// redo returns to this saved revision. Callers must decide whether discarding
// local edits is authorized before invoking Reload.
func (b *Buffer) Reload(text string) (ChangeEvent, error) {
	next, err := New(text)
	if err != nil {
		return ChangeEvent{}, err
	}
	old := b.Text()
	text = next.Text()
	if old == text {
		b.MarkSaved()
		return ChangeEvent{Version: b.version}, nil
	}
	r := Range{Position{}, Position{len(b.lines) - 1, units(b.lines[len(b.lines)-1])}}
	before, oldEOL := b.selection, b.eol
	b.lines, b.eol = next.lines, next.eol
	clamp := func(p Position) Position {
		line := max(0, min(p.Line, len(b.lines)-1))
		column := max(0, min(p.Character, units(b.lines[line])))
		for column > 0 {
			if _, err := byteColumn(b.lines[line], column); err == nil {
				break
			}
			column--
		}
		return Position{line, column}
	}
	b.selection = Selection{clamp(before.Anchor), clamp(before.Active)}
	b.nextRevision++
	b.record(history{Position{}, old, text, before, b.selection, b.revision, b.nextRevision, oldEOL, b.eol})
	b.revision = b.nextRevision
	b.MarkSaved()
	b.version++
	return ChangeEvent{b.version, []Change{{r, units(old), text}}}, nil
}
func (b *Buffer) ReplaceSelection(text string) (ChangeEvent, error) {
	r := b.selection.Range()
	p := endOf(r.Start, text)
	return b.Apply([]Edit{{r, text}}, &Selection{p, p})
}
func (b *Buffer) Undo() (ChangeEvent, bool) {
	if len(b.undo) == 0 {
		return ChangeEvent{}, false
	}
	item := b.undo[len(b.undo)-1]
	b.undo[len(b.undo)-1] = history{}
	b.undo = b.undo[:len(b.undo)-1]
	b.historyBytes -= len(item.removed) + len(item.inserted)
	r := Range{item.start, endOf(item.start, item.inserted)}
	b.splice(r, item.removed)
	b.eol = item.beforeEOL
	b.selection = item.before
	b.revision = item.beforeRevision
	b.version++
	b.redo = append(b.redo, item)
	return ChangeEvent{b.version, []Change{{r, units(item.inserted), item.removed}}}, true
}
func (b *Buffer) Redo() (ChangeEvent, bool) {
	if len(b.redo) == 0 {
		return ChangeEvent{}, false
	}
	item := b.redo[len(b.redo)-1]
	b.redo[len(b.redo)-1] = history{}
	b.redo = b.redo[:len(b.redo)-1]
	r := Range{item.start, endOf(item.start, item.removed)}
	b.splice(r, item.inserted)
	b.eol = item.afterEOL
	b.selection = item.after
	b.revision = item.afterRevision
	b.version++
	b.undo = append(b.undo, item)
	b.historyBytes += len(item.removed) + len(item.inserted)
	return ChangeEvent{b.version, []Change{{r, units(item.removed), item.inserted}}}, true
}
