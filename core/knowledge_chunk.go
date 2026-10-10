package core

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	DefaultKnowledgeChunkTokens   = 650
	DefaultKnowledgeChunkOverlap  = 80
	DefaultKnowledgeSearchLimit   = 5
	MaxKnowledgeSearchLimit       = 20
	defaultKnowledgeCandidatePool = 10
)

type KnowledgeChunk struct {
	Content    string `json:"content"`
	Index      int    `json:"index"`
	TokenCount int    `json:"tokenCount"`
}

// CountKnowledgeTokens provides a deterministic tokenizer-independent
// estimate suitable for chunk sizing and observability. Model providers still
// report their authoritative token usage for generation calls.
func CountKnowledgeTokens(text string) int {
	count := 0
	inWord := false
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			inWord = false
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			count++
			inWord = false
		case !inWord:
			count++
			inWord = true
		}
	}
	return count
}

// ChunkKnowledgeText splits normalized text on paragraph boundaries and only
// falls back to word windows when a single paragraph is too large.
func ChunkKnowledgeText(text string, maxTokens, overlapTokens int) ([]KnowledgeChunk, error) {
	text = normalizeKnowledgeText(text)
	if text == "" {
		return nil, nil
	}
	if maxTokens <= 0 {
		maxTokens = DefaultKnowledgeChunkTokens
	}
	if overlapTokens < 0 || overlapTokens >= maxTokens {
		return nil, fmt.Errorf("chunk overlap must be between 0 and %d", maxTokens-1)
	}

	paragraphs := strings.Split(text, "\n\n")
	chunks := make([]KnowledgeChunk, 0)
	current := make([]string, 0)
	currentTokens := 0
	currentIsOverlap := false
	reset := func() {
		current = current[:0]
		currentTokens = 0
		currentIsOverlap = false
	}

	flush := func() {
		content := strings.TrimSpace(strings.Join(current, "\n\n"))
		if content == "" || currentIsOverlap {
			reset()
			return
		}
		chunks = append(chunks, KnowledgeChunk{
			Content:    content,
			Index:      len(chunks),
			TokenCount: CountKnowledgeTokens(content),
		})

		if overlapTokens == 0 {
			reset()
			return
		}
		current = []string{knowledgeTokenSuffix(content, overlapTokens)}
		currentTokens = CountKnowledgeTokens(current[0])
		currentIsOverlap = true
	}

	for _, paragraph := range paragraphs {
		rest := strings.TrimSpace(paragraph)
		for rest != "" {
			capacity := maxTokens - currentTokens
			if capacity <= 0 {
				flush()
				capacity = maxTokens - currentTokens
			}

			end := knowledgeTokenPrefixEnd(rest, capacity)
			if end <= 0 {
				_, size := utf8.DecodeRuneInString(rest)
				end = size
			}
			part := strings.TrimSpace(rest[:end])
			if part != "" {
				current = append(current, part)
				currentTokens = CountKnowledgeTokens(strings.Join(current, "\n\n"))
			}
			currentIsOverlap = false
			rest = strings.TrimSpace(rest[end:])
			if rest != "" {
				flush()
			}
		}
	}
	if currentTokens > 0 && !currentIsOverlap {
		flush()
	}

	return chunks, nil
}

// knowledgeTokenPrefixEnd returns the byte offset of the largest prefix that
// fits within maxTokens according to CountKnowledgeTokens.
func knowledgeTokenPrefixEnd(value string, maxTokens int) int {
	count := 0
	inWord := false
	end := 0
	for index, r := range value {
		switch {
		case unicode.IsSpace(r):
			inWord = false
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			count++
			inWord = false
		case !inWord:
			count++
			inWord = true
		}
		if count > maxTokens {
			return index
		}
		end = index + utf8.RuneLen(r)
	}
	return end
}

func knowledgeTokenSuffix(value string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	starts := make([]int, 0, CountKnowledgeTokens(value))
	inWord := false
	for index, r := range value {
		switch {
		case unicode.IsSpace(r):
			inWord = false
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			starts = append(starts, index)
			inWord = false
		case !inWord:
			starts = append(starts, index)
			inWord = true
		}
	}
	if len(starts) <= maxTokens {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(value[starts[len(starts)-maxTokens]:])
}
