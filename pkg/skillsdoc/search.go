// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package skillsdoc

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Okapi BM25 parameters (the commonly used defaults).
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// Field weights. A term in a skill's name or description counts for more than
// the same term somewhere in the body (the usual "BM25F" simplification:
// weighted term frequencies are summed into one pseudo-frequency per term).
const (
	weightName        = 3.0
	weightDescription = 2.0
	weightBody        = 1.0
)

// SearchResult is a skill doc together with its BM25 relevance score.
type SearchResult struct {
	Doc   *SkillDoc
	Score float64
}

// Search ranks docs against query using BM25 and returns only docs that match
// at least one query term, best first. Ties are broken by skill name so the
// output is deterministic. An empty query (or one without any searchable
// terms) returns no results.
//
// The corpus is small and fixed (embedded in the binary), so the index is
// simply built on every call.
func Search(docs []*SkillDoc, query string) []SearchResult {
	terms := dedupe(Tokenize(query))
	if len(terms) == 0 || len(docs) == 0 {
		return nil
	}

	type indexed struct {
		tf  map[string]float64 // weighted term frequencies
		len float64            // weighted document length
	}
	idx := make([]indexed, len(docs))
	df := make(map[string]int) // number of docs containing each term
	var totalLen float64
	for i, d := range docs {
		tf := make(map[string]float64)
		var l float64
		add := func(text string, w float64) {
			for _, t := range Tokenize(text) {
				tf[t] += w
				l += w
			}
		}
		add(d.Name, weightName)
		add(d.Description, weightDescription)
		add(bodyOf(d.Content), weightBody)
		idx[i] = indexed{tf: tf, len: l}
		totalLen += l
		for t := range tf {
			df[t]++
		}
	}
	avgLen := totalLen / float64(len(docs))
	n := float64(len(docs))

	var out []SearchResult
	for i, d := range docs {
		var score float64
		for _, t := range terms {
			f := idx[i].tf[t]
			if f == 0 {
				continue
			}
			nt := float64(df[t])
			// Lucene-style IDF; always >= 0, so very common terms never
			// push a score negative.
			idf := math.Log(1 + (n-nt+0.5)/(nt+0.5))
			norm := f + bm25K1*(1-bm25B+bm25B*idx[i].len/avgLen)
			score += idf * f * (bm25K1 + 1) / norm
		}
		if score > 0 {
			out = append(out, SearchResult{Doc: d, Score: score})
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Score != out[b].Score {
			return out[a].Score > out[b].Score
		}
		return out[a].Doc.Name < out[b].Doc.Name
	})
	return out
}

// Tokenize lower-cases text and splits it on anything that is not a letter or
// digit, so "ivcap-job-create" becomes ["ivcap" "job" "create"]. No stemming
// or stop-word removal is done: BM25's IDF already discounts common words.
func Tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// bodyOf returns content without its leading YAML front-matter (name and
// description are indexed separately with their own weights).
func bodyOf(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	nl := strings.IndexByte(content, '\n')
	if nl < 0 {
		return content
	}
	rest := content[nl+1:]
	for _, delim := range []string{"\n---\n", "\n---\r\n"} {
		if i := strings.Index(rest, delim); i >= 0 {
			return rest[i+len(delim):]
		}
	}
	return content
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := in[:0:0]
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
