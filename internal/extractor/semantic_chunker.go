package extractor

import (
	"regexp"
	"strings"
)

// TextSegment represents a coherent chunk of text (such as a paragraph,
// sentence, or word) along with its trailing whitespace/formatting separator.
type TextSegment struct {
	Text      string
	Separator string
}

// SemanticChunking segments raw text into chunks of at most maxChunkSize,
// keeping an overlap of approximately overlapSize characters between subsequent chunks.
// It prioritizes paragraph boundaries (\n\n), then sentence boundaries (. ! ?),
// and falls back to word boundaries (spaces) if a sentence exceeds the limit.
func SemanticChunking(text string, maxChunkSize, overlapSize int) []string {
	// Standardize line endings
	normalized := strings.ReplaceAll(text, "\r\n", "\n")

	// If the entire text fits, return it as a single chunk
	if len(normalized) <= maxChunkSize {
		trimmed := strings.TrimSpace(normalized)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	}

	// 1. Split into paragraphs
	paragraphs := strings.Split(normalized, "\n\n")
	var segments []TextSegment

	for i, p := range paragraphs {
		sep := "\n\n"
		if i == len(paragraphs)-1 {
			sep = ""
		}

		if len(p)+len(sep) <= maxChunkSize {
			segments = append(segments, TextSegment{Text: p, Separator: sep})
		} else {
			segments = append(segments, splitParagraph(p, sep, maxChunkSize)...)
		}
	}

	// 2. Group segments into chunks with overlap
	return groupSegmentsIntoChunks(segments, maxChunkSize, overlapSize)
}

// splitParagraph divides a paragraph into sentences using standard punctuation marks (. ! ?)
func splitParagraph(p string, parentSeparator string, maxChunkSize int) []TextSegment {
	// Regex matching sentence endings followed by whitespace or end of string.
	re := regexp.MustCompile(`[^.!?]+[.!?]+(\s+|$)`)
	matches := re.FindAllStringIndex(p, -1)

	var segments []TextSegment
	if len(matches) == 0 {
		return splitSentence(p, parentSeparator, maxChunkSize)
	}

	lastIdx := 0
	for i, match := range matches {
		_, end := match[0], match[1]
		sentenceWithSpaces := p[lastIdx:end]
		trimmedSentence := strings.TrimRight(sentenceWithSpaces, " \t\n\r")
		spaces := sentenceWithSpaces[len(trimmedSentence):]

		sep := spaces
		if i == len(matches)-1 {
			sep = parentSeparator
		}

		if len(trimmedSentence)+len(sep) <= maxChunkSize {
			segments = append(segments, TextSegment{Text: trimmedSentence, Separator: sep})
		} else {
			segments = append(segments, splitSentence(trimmedSentence, sep, maxChunkSize)...)
		}
		lastIdx = end
	}

	// Check if any trailing text remains after the last matched sentence boundary
	if lastIdx < len(p) {
		remaining := p[lastIdx:]
		trimmedRemaining := strings.TrimRight(remaining, " \t\n\r")
		if trimmedRemaining != "" {
			if len(trimmedRemaining)+len(parentSeparator) <= maxChunkSize {
				segments = append(segments, TextSegment{Text: trimmedRemaining, Separator: parentSeparator})
			} else {
				segments = append(segments, splitSentence(trimmedRemaining, parentSeparator, maxChunkSize)...)
			}
		}
	}

	return segments
}

// splitSentence divides a sentence into individual words (fallback when sentence is larger than limit)
func splitSentence(s string, parentSeparator string, maxChunkSize int) []TextSegment {
	words := strings.Fields(s)
	var segments []TextSegment
	if len(words) == 0 {
		return nil
	}

	for i, word := range words {
		sep := " "
		if i == len(words)-1 {
			sep = parentSeparator
		}
		segments = append(segments, TextSegment{Text: word, Separator: sep})
	}
	return segments
}

// groupSegmentsIntoChunks groups the atomic TextSegments into logical chunks up to maxChunkSize with sliding overlap
func groupSegmentsIntoChunks(segments []TextSegment, maxChunkSize, overlapSize int) []string {
	var chunks []string
	n := len(segments)
	if n == 0 {
		return nil
	}

	currentStart := 0
	for currentStart < n {
		var chunkBuilder strings.Builder
		currentLen := 0
		endIdx := currentStart

		for endIdx < n {
			seg := segments[endIdx]
			segLen := len(seg.Text) + len(seg.Separator)

			if currentLen+segLen > maxChunkSize {
				// Force consume at least one segment to avoid getting stuck if a single segment is huge
				if endIdx == currentStart {
					chunkBuilder.WriteString(seg.Text)
					chunkBuilder.WriteString(seg.Separator)
					endIdx++
				}
				break
			}
			chunkBuilder.WriteString(seg.Text)
			chunkBuilder.WriteString(seg.Separator)
			currentLen += segLen
			endIdx++
		}

		trimmedChunk := strings.TrimSpace(chunkBuilder.String())
		if trimmedChunk != "" {
			chunks = append(chunks, trimmedChunk)
		}

		if endIdx >= n {
			break
		}

		// Calculate overlap back-tracking from the end of the current chunk
		nextStart := endIdx
		if overlapSize > 0 {
			overlapLen := 0
			for j := endIdx - 1; j > currentStart; j-- {
				seg := segments[j]
				overlapLen += len(seg.Text) + len(seg.Separator)
				if overlapLen >= overlapSize {
					nextStart = j
					break
				}
				// Use the furthest available boundary as a fallback for maximum possible overlap
				nextStart = j
			}
		}

		// Safeguard: Ensure we always advance forward to prevent infinite loops
		if nextStart <= currentStart {
			nextStart = currentStart + 1
		}

		currentStart = nextStart
	}

	return chunks
}
