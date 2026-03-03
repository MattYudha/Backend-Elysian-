package rag

import (
	"fmt"
	"regexp"
	"strings"
)

// MarkdownChunk is a single chunk produced by the Markdown-aware chunker.
// It carries both the raw content AND the full header breadcrumb for LLM context injection.
type MarkdownChunk struct {
	// HeaderPath is the full parent context, e.g. "Policy Document > Section 3 > Leave Types"
	// This is injected as a prefix into every chunk so LLMs understand partial context.
	HeaderPath string
	// Content is the raw text of this chunk (without header injection)
	Content string
	// FullContent = HeaderPath + "\n" + Content — this is what goes into the embedding
	FullContent string
	// Index is the sequential chunk number within the document
	Index int
}

// headerRegex matches Markdown headers: # H1, ## H2, ### H3, etc.
var headerRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// tableRowRegex matches a Markdown table row (starts with |)
var tableRowRegex = regexp.MustCompile(`^\s*\|`)

// MarkdownAwareChunker splits Markdown text into semantically coherent chunks.
//
// Rules:
//  1. Primary split boundary = Markdown headers (# ## ###)
//  2. Tables are NEVER split mid-row — an entire table is kept as one unit
//     (if a single table exceeds maxChunkSize, it is emitted as one oversized chunk with a warning)
//  3. Non-table text within a section is word-boundary chunked at maxChunkSize
//  4. Every chunk is prefixed with its header breadcrumb path for LLM context
//
// Parameters:
//   - text: Full Markdown text output from Docling
//   - maxChunkSize: Target character limit per chunk (default: 1000)
//   - chunkOverlap: Characters of trailing overlap to carry into next chunk (default: 0 for Markdown)
func MarkdownAwareChunker(text string, maxChunkSize int) []MarkdownChunk {
	if maxChunkSize <= 0 {
		maxChunkSize = 1000
	}

	lines := strings.Split(text, "\n")

	// Phase 1: Parse the document into logical sections grouped by header hierarchy
	type section struct {
		headers      []string // breadcrumb: ["Doc Title", "Section 1", "Sub-section A"]
		contentLines []string // raw content lines under this header
	}

	var sections []section
	currentHeaders := []string{}
	currentContent := []string{}

	flushSection := func() {
		if len(currentContent) > 0 {
			sections = append(sections, section{
				headers:      append([]string{}, currentHeaders...),
				contentLines: append([]string{}, currentContent...),
			})
			currentContent = nil
		}
	}

	for _, line := range lines {
		if m := headerRegex.FindStringSubmatch(line); m != nil {
			flushSection()
			level := len(m[1]) // number of # characters
			headerText := strings.TrimSpace(m[2])

			// Trim the breadcrumb to the current level
			if level <= len(currentHeaders) {
				currentHeaders = currentHeaders[:level-1]
			}
			currentHeaders = append(currentHeaders, headerText)
		} else {
			currentContent = append(currentContent, line)
		}
	}
	flushSection() // Flush any remaining content

	// Phase 2: Chunk each section, preserving tables
	var chunks []MarkdownChunk
	chunkIndex := 0

	for _, sec := range sections {
		headerPath := strings.Join(sec.headers, " > ")

		// Separate the section content into blocks: plain-text sequences and table sequences
		type block struct {
			isTable bool
			lines   []string
		}
		var blocks []block

		i := 0
		for i < len(sec.contentLines) {
			if tableRowRegex.MatchString(sec.contentLines[i]) {
				// Collect the entire table as one atomic block
				var tableLines []string
				for i < len(sec.contentLines) && tableRowRegex.MatchString(sec.contentLines[i]) {
					tableLines = append(tableLines, sec.contentLines[i])
					i++
				}
				blocks = append(blocks, block{isTable: true, lines: tableLines})
			} else {
				// Collect plain text lines until the next table or end
				var textLines []string
				for i < len(sec.contentLines) && !tableRowRegex.MatchString(sec.contentLines[i]) {
					textLines = append(textLines, sec.contentLines[i])
					i++
				}
				blocks = append(blocks, block{isTable: false, lines: textLines})
			}
		}

		// Phase 3: Emit chunks from blocks
		for _, blk := range blocks {
			raw := strings.TrimSpace(strings.Join(blk.lines, "\n"))
			if raw == "" {
				continue
			}

			if blk.isTable {
				// Tables are NEVER split — emit as one chunk regardless of size
				chunk := buildChunk(headerPath, raw, chunkIndex)
				chunks = append(chunks, chunk)
				chunkIndex++
			} else {
				// Plain text: word-boundary chunking at maxChunkSize
				textChunks := splitByWordBoundary(raw, maxChunkSize)
				for _, tc := range textChunks {
					if strings.TrimSpace(tc) == "" {
						continue
					}
					chunk := buildChunk(headerPath, tc, chunkIndex)
					chunks = append(chunks, chunk)
					chunkIndex++
				}
			}
		}
	}

	return chunks
}

// buildChunk constructs a MarkdownChunk with the parent header path injected as context.
// The FullContent (what gets embedded) = "[Context: path]\n\ncontent"
// This ensures partial chunks always carry their document position for the LLM.
func buildChunk(headerPath, content string, index int) MarkdownChunk {
	var fullContent string
	if headerPath != "" {
		fullContent = fmt.Sprintf("[Context: %s]\n\n%s", headerPath, content)
	} else {
		fullContent = content
	}
	return MarkdownChunk{
		HeaderPath:  headerPath,
		Content:     content,
		FullContent: fullContent,
		Index:       index,
	}
}

// splitByWordBoundary breaks a string into slices of at most maxLen characters
// without breaking in the middle of a word.
func splitByWordBoundary(text string, maxLen int) []string {
	words := strings.Fields(text)
	var chunks []string
	var current strings.Builder

	for _, word := range words {
		// +1 for the space
		if current.Len()+len(word)+1 > maxLen && current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(word)
	}
	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}
	return chunks
}
