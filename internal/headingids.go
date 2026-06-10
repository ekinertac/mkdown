// headingids.go — a faster drop-in replacement for goldmark's heading-ID
// generator (parser.IDs), wired in per-conversion via parser.WithIDs in
// converter.go.
//
// Why: goldmark's default ids.Generate resolves a duplicate slug by linearly
// probing "slug-1", "slug-2", ... starting from 1 on every collision, each
// probe allocating a fresh fmt.Sprintf string. For a document with N identical
// headings that is O(N^2) Sprintf calls, and profiling showed it as the single
// largest allocator (~31% of all allocations) — it dominates on the kinds of
// docs that repeat headings heavily: API references ("Parameters", "Returns",
// "Example") and changelogs ("Bug Fixes", "Features" per release).
//
// fastIDs keeps the same `values` set the default uses (so behavior is
// identical) but additionally remembers, per base slug, the next suffix worth
// trying. Because IDs are never removed once assigned, every suffix below that
// hint is already taken — so starting the probe there yields the exact same
// smallest-free suffix the default would return, just without re-scanning from
// 1. Output is therefore byte-identical; only the work to get there shrinks.
//
// The slugification below is copied verbatim from goldmark's implementation so
// the produced IDs match exactly.
package internal

import (
	"fmt"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

type fastIDs struct {
	values map[string]bool // every ID handed out so far (same as goldmark's)
	next   map[string]int  // base slug -> next suffix to try (optimization only)
}

func newFastIDs() *fastIDs {
	return &fastIDs{
		values: map[string]bool{},
		next:   map[string]int{},
	}
}

// slug mirrors goldmark/parser.(*ids).Generate's slugification exactly.
func (s *fastIDs) slug(value []byte, kind ast.NodeKind) []byte {
	value = util.TrimLeftSpace(value)
	value = util.TrimRightSpace(value)
	result := []byte{}
	for i := 0; i < len(value); {
		v := value[i]
		l := util.UTF8Len(v)
		i += int(l)
		if l != 1 {
			continue
		}
		if util.IsAlphaNumeric(v) {
			if 'A' <= v && v <= 'Z' {
				v += 'a' - 'A'
			}
			result = append(result, v)
		} else if util.IsSpace(v) || v == '-' || v == '_' {
			result = append(result, '-')
		}
	}
	if len(result) == 0 {
		if kind == ast.KindHeading {
			result = []byte("heading")
		} else {
			result = []byte("id")
		}
	}
	return result
}

func (s *fastIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	result := s.slug(value, kind)
	if !s.values[util.BytesToReadOnlyString(result)] {
		s.values[util.BytesToReadOnlyString(result)] = true
		return result
	}
	// Duplicate: probe from the remembered suffix rather than from 1. Every
	// suffix below s.next[base] was already returned (IDs are never freed), so
	// this lands on the same value the linear-from-1 scan would, with the map
	// check still guarding against suffixes taken via Put or explicit IDs.
	base := string(result)
	i := s.next[base]
	if i < 1 {
		i = 1
	}
	for {
		newResult := fmt.Sprintf("%s-%d", result, i)
		if !s.values[newResult] {
			s.values[newResult] = true
			s.next[base] = i + 1
			return []byte(newResult)
		}
		i++
	}
}

func (s *fastIDs) Put(value []byte) {
	s.values[util.BytesToReadOnlyString(value)] = true
}
