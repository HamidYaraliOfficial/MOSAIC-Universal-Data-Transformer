package parser

import (
	"bufio"
	"encoding/csv"
	"io"
	"strings"

	"mosaic/internal/schema"
)

func init() {
	Register(&csvParser{})
}

// csvParser implements CSV, TSV and arbitrary Custom-Delimited files. The
// same implementation backs all three since they only differ by delimiter,
// which Sniff / delimiter-detection resolves automatically.
type csvParser struct{}

func (csvParser) Name() string { return "csv" }

func (csvParser) Sniff(filename string, head []byte) float64 {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return 0.95
	case strings.HasSuffix(lower, ".tsv"):
		return 0.9
	case strings.HasSuffix(lower, ".txt"):
		if looksDelimited(head) {
			return 0.4
		}
		return 0.1
	default:
		if looksDelimited(head) {
			return 0.3
		}
		return 0
	}
}

func looksDelimited(head []byte) bool {
	line := string(head)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	for _, d := range []byte{',', '\t', ';', '|'} {
		if strings.Count(line, string(d)) >= 1 {
			return true
		}
	}
	return false
}

// detectDelimiter scores common delimiters by consistency across the first
// few lines (the delimiter that yields the same field count on every line
// wins) — the same heuristic the in-app Data Profiler surfaces to the user.
func detectDelimiter(sample string) rune {
	candidates := []rune{',', '\t', ';', '|'}
	lines := strings.Split(sample, "\n")
	if len(lines) > 5 {
		lines = lines[:5]
	}
	bestDelim, bestScore := ',', -1
	for _, d := range candidates {
		counts := map[int]int{}
		for _, l := range lines {
			if l == "" {
				continue
			}
			counts[strings.Count(l, string(d))]++
		}
		// Score = how many lines agree on the modal field count, weighted
		// by that count itself (more fields = more informative match).
		modal, modalCount := 0, 0
		for count, freq := range counts {
			if freq > modalCount {
				modal, modalCount = count, freq
			}
		}
		score := modalCount * (modal + 1)
		if score > bestScore && modal > 0 {
			bestDelim, bestScore = d, score
		}
	}
	return bestDelim
}

func (p csvParser) Parse(r io.Reader, opt Options) (Result, error) {
	res := Result{Format: "csv"}
	buffered := bufio.NewReaderSize(r, 64*1024)
	head, _ := buffered.Peek(4096)
	delim := opt.Delimiter
	if delim == 0 {
		delim = detectDelimiter(string(head))
	}

	cr := csv.NewReader(buffered)
	cr.Comma = delim
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true

	var columns []string
	first := true
	limit := opt.SampleLimit
	count := 0
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows rather than aborting the whole import
		}
		if first {
			first = false
			if opt.HasHeader {
				columns = record
				continue
			}
			columns = generateColumnNames(len(record))
		}
		row := make(schema.Row, len(columns))
		for i, col := range columns {
			if i < len(record) {
				row[col] = record[i]
			} else {
				row[col] = nil
			}
		}
		res.Rows = append(res.Rows, row)
		count++
		if limit > 0 && count >= limit {
			break
		}
	}
	res.Columns = columns
	return res, nil
}

func (p csvParser) Stream(r io.Reader, opt Options, handle RowHandler) error {
	buffered := bufio.NewReaderSize(r, 256*1024)
	head, _ := buffered.Peek(4096)
	delim := opt.Delimiter
	if delim == 0 {
		delim = detectDelimiter(string(head))
	}
	cr := csv.NewReader(buffered)
	cr.Comma = delim
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true

	var columns []string
	first := true
	for {
		record, err := cr.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			continue
		}
		if first {
			first = false
			if opt.HasHeader {
				columns = record
				continue
			}
			columns = generateColumnNames(len(record))
		}
		row := make(schema.Row, len(columns))
		for i, col := range columns {
			if i < len(record) {
				row[col] = record[i]
			}
		}
		if err := handle(row); err != nil {
			return err
		}
	}
}

func generateColumnNames(n int) []string {
	cols := make([]string, n)
	for i := range cols {
		cols[i] = "column_" + itoa(i+1)
	}
	return cols
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
