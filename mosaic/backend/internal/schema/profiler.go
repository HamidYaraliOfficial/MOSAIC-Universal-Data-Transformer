package schema

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ColumnProfile is the output of the Data Profiler for a single column:
// everything the Import screen needs to show in the profiling summary
// (null rate, uniqueness, detected pattern, min/max, etc).
type ColumnProfile struct {
	InferredType  DataType       `json:"inferredType"`
	NullCount     int            `json:"nullCount"`
	NullRate      float64        `json:"nullRate"`
	DistinctCount int            `json:"distinctCount"`
	DuplicateRate float64        `json:"duplicateRate"`
	Min           string         `json:"min,omitempty"`
	Max           string         `json:"max,omitempty"`
	MeanLength    float64        `json:"meanLength"`
	FormatPattern string         `json:"formatPattern,omitempty"`
	SampleValues  []string       `json:"sampleValues,omitempty"`
	TypeVotes     map[string]int `json:"typeVotes,omitempty"`
}

// Report is the full result of profiling a dataset: an inferred schema plus
// per-column statistics and an overall row count / sample.
type Report struct {
	Schema     Schema     `json:"schema"`
	RowCount   int        `json:"rowCount"`
	SampleRows []Row      `json:"sampleRows"`
	Duration   float64    `json:"durationMs"`
	Warnings   []string   `json:"warnings,omitempty"`
	Columns    []string   `json:"columns"`
}

var (
	reInt      = regexp.MustCompile(`^-?\d+$`)
	reFloat    = regexp.MustCompile(`^-?\d+\.\d+$`)
	reBool     = regexp.MustCompile(`(?i)^(true|false|yes|no|0|1)$`)
	reUUID     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	reDate     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reDateTime = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)
	reTime     = regexp.MustCompile(`^\d{2}:\d{2}(:\d{2})?$`)
)

// Profile runs full column-type inference, null/duplicate/uniqueness
// statistics and min/max/pattern detection over the given rows. It is the
// Go implementation backing the "Data Profiler" step that runs automatically
// on every Import in the MOSAIC UI.
func Profile(columns []string, rows []Row, sampleLimit int) Report {
	start := time.Now()
	report := Report{Columns: columns, RowCount: len(rows)}

	// Track distinct full-row signatures for duplicate-rate.
	seenRows := make(map[string]int, len(rows))
	for _, r := range rows {
		seenRows[rowSignature(columns, r)]++
	}
	dupRows := 0
	for _, c := range seenRows {
		if c > 1 {
			dupRows += c - 1
		}
	}

	cols := make([]Column, 0, len(columns))
	for _, name := range columns {
		values := make([]string, 0, len(rows))
		nullCount := 0
		distinct := make(map[string]int)
		votes := map[string]int{"integer": 0, "float": 0, "boolean": 0, "date": 0, "datetime": 0, "time": 0, "uuid": 0, "string": 0}

		for _, r := range rows {
			raw, ok := r[name]
			s := toStr(raw)
			if !ok || raw == nil || strings.TrimSpace(s) == "" {
				nullCount++
				continue
			}
			values = append(values, s)
			distinct[s]++
			votes[classify(s)]++
		}

		inferred := majorityType(votes)
		mn, mx := minMax(values, inferred)

		var meanLen float64
		if len(values) > 0 {
			total := 0
			for _, v := range values {
				total += len(v)
			}
			meanLen = float64(total) / float64(len(values))
		}

		sample := values
		if len(sample) > 8 {
			sample = sample[:8]
		}

		profile := &ColumnProfile{
			InferredType:  inferred,
			NullCount:     nullCount,
			NullRate:      safeDiv(nullCount, len(rows)),
			DistinctCount: len(distinct),
			DuplicateRate: safeDiv(len(values)-len(distinct), max1(len(values))),
			Min:           mn,
			Max:           mx,
			MeanLength:    meanLen,
			FormatPattern: detectPattern(values),
			SampleValues:  sample,
			TypeVotes:     votes,
		}

		cols = append(cols, Column{
			Name:     name,
			Type:     inferred,
			Nullable: nullCount > 0,
			Stats:    profile,
		})
	}

	report.Schema = Schema{Columns: cols}
	limit := sampleLimit
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	report.SampleRows = rows[:limit]
	if len(rows) > 0 {
		report.Warnings = duplicateWarning(dupRows, len(rows))
	}
	report.Duration = float64(time.Since(start).Microseconds()) / 1000.0
	return report
}

func duplicateWarning(dup, total int) []string {
	if dup == 0 {
		return nil
	}
	rate := float64(dup) / float64(total) * 100
	if rate < 1 {
		return nil
	}
	return []string{"duplicate rows detected: " + strconv.FormatFloat(rate, 'f', 1, 64) + "%"}
}

func classify(s string) string {
	switch {
	case reUUID.MatchString(s):
		return "uuid"
	case reDateTime.MatchString(s):
		return "datetime"
	case reDate.MatchString(s):
		return "date"
	case reTime.MatchString(s):
		return "time"
	case reInt.MatchString(s):
		return "integer"
	case reFloat.MatchString(s):
		return "float"
	case reBool.MatchString(s):
		return "boolean"
	default:
		return "string"
	}
}

func majorityType(votes map[string]int) DataType {
	best, bestCount := "string", -1
	for t, c := range votes {
		if c > bestCount {
			best, bestCount = t, c
		}
	}
	switch best {
	case "integer":
		return TypeInteger
	case "float":
		return TypeFloat
	case "boolean":
		return TypeBoolean
	case "date":
		return TypeDate
	case "datetime":
		return TypeDateTime
	case "time":
		return TypeTime
	case "uuid":
		return TypeUUID
	default:
		return TypeString
	}
}

func minMax(values []string, t DataType) (string, string) {
	if len(values) == 0 {
		return "", ""
	}
	if t == TypeInteger || t == TypeFloat {
		var mn, mx float64
		mn, mx = math.MaxFloat64, -math.MaxFloat64
		for _, v := range values {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				continue
			}
			if f < mn {
				mn = f
			}
			if f > mx {
				mx = f
			}
		}
		return strconv.FormatFloat(mn, 'f', -1, 64), strconv.FormatFloat(mx, 'f', -1, 64)
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted[0], sorted[len(sorted)-1]
}

// detectPattern produces a coarse, human-readable format signature (e.g.
// "AAA-9999" for "ABC-1234") used to flag Inconsistent Formatting quickly.
func detectPattern(values []string) string {
	if len(values) == 0 {
		return ""
	}
	patterns := make(map[string]int)
	for _, v := range values {
		if len(v) > 32 {
			continue
		}
		var b strings.Builder
		for _, r := range v {
			switch {
			case r >= '0' && r <= '9':
				b.WriteByte('9')
			case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
				b.WriteByte('A')
			default:
				b.WriteRune(r)
			}
		}
		patterns[b.String()]++
	}
	best, bestCount := "", 0
	for p, c := range patterns {
		if c > bestCount {
			best, bestCount = p, c
		}
	}
	return best
}

func rowSignature(columns []string, r Row) string {
	var b strings.Builder
	for _, c := range columns {
		b.WriteString(toStr(r[c]))
		b.WriteByte('\x1f')
	}
	return b.String()
}

func toStr(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case bool:
		return strconv.FormatBool(t)
	default:
		return ""
	}
}

func safeDiv(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}
