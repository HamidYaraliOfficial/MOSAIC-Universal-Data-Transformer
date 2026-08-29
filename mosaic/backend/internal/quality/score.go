// Package quality implements the Data Quality Score shown in the Data
// Cleaning Studio and Report/Preview Studio: a configurable weighted score
// across five dimensions, each broken down per column with concrete issues.
package quality

import (
	"fmt"
	"strconv"
	"strings"

	"mosaic/internal/schema"
)

// Weights lets the user tune how much each dimension counts toward the
// overall score, exposed in Settings for the Data Quality Score card.
type Weights struct {
	Completeness float64 `json:"completeness"`
	Validity     float64 `json:"validity"`
	Uniqueness   float64 `json:"uniqueness"`
	Consistency  float64 `json:"consistency"`
	Accuracy     float64 `json:"accuracy"`
}

// DefaultWeights gives every dimension equal weight.
func DefaultWeights() Weights { return Weights{0.2, 0.2, 0.2, 0.2, 0.2} }

// Issue is one concrete, actionable data quality problem surfaced to the
// user (e.g. "12% of `email` values are empty").
type Issue struct {
	Column     string `json:"column"`
	Dimension  string `json:"dimension"`
	Message    string `json:"message"`
	Severity   string `json:"severity"` // info|warning|critical
	AffectedPct float64 `json:"affectedPct"`
}

// Report is the full quality score result for a dataset.
type Report struct {
	Overall      float64            `json:"overall"`
	Completeness float64            `json:"completeness"`
	Validity     float64            `json:"validity"`
	Uniqueness   float64            `json:"uniqueness"`
	Consistency  float64            `json:"consistency"`
	Accuracy     float64            `json:"accuracy"`
	Issues       []Issue            `json:"issues"`
}

// Score analyzes rows against the given schema and returns a weighted
// Data Quality Score plus a flat list of concrete issues for the UI to
// group and display per column.
func Score(sch schema.Schema, rows []schema.Row, w Weights) Report {
	if len(rows) == 0 {
		return Report{}
	}
	var issues []Issue
	var completenessSum, validitySum, uniquenessSum, consistencySum, accuracySum float64
	n := float64(len(sch.Columns))
	if n == 0 {
		return Report{}
	}

	for _, col := range sch.Columns {
		values := make([]string, 0, len(rows))
		nullCount := 0
		distinct := map[string]int{}
		invalid := 0
		patternMismatch := 0
		dominantPattern := ""
		if col.Stats != nil {
			dominantPattern = col.Stats.FormatPattern
		}

		for _, r := range rows {
			v, ok := r[col.Name]
			s := toStr(v)
			if !ok || v == nil || strings.TrimSpace(s) == "" {
				nullCount++
				continue
			}
			values = append(values, s)
			distinct[s]++
			if !typeMatches(s, col.Type) {
				invalid++
			}
			if dominantPattern != "" && patternOf(s) != dominantPattern {
				patternMismatch++
			}
		}

		total := len(rows)
		completeness := 1 - safeDiv(nullCount, total)
		validity := 1.0
		if len(values) > 0 {
			validity = 1 - safeDiv(invalid, len(values))
		}
		uniqueness := 1.0
		if col.Unique && len(values) > 0 {
			uniqueness = safeDiv(len(distinct), len(values))
		}
		consistency := 1.0
		if len(values) > 0 && dominantPattern != "" {
			consistency = 1 - safeDiv(patternMismatch, len(values))
		}
		accuracy := validity // proxy: MOSAIC has no ground truth, so accuracy
		// tracks validity plus range-constraint checks (below).
		for _, c := range col.Constraints {
			viol := countConstraintViolations(values, c)
			if len(values) > 0 {
				accuracy -= safeDiv(viol, len(values))
			}
			if viol > 0 {
				issues = append(issues, Issue{
					Column: col.Name, Dimension: "accuracy",
					Message:     fmt.Sprintf("%d values violate constraint %s %s", viol, c.Kind, c.Value),
					Severity:    severityFor(safeDiv(viol, max1(len(values)))),
					AffectedPct: safeDiv(viol, max1(len(values))) * 100,
				})
			}
		}
		accuracy = clamp01(accuracy)

		completenessSum += completeness
		validitySum += validity
		uniquenessSum += uniqueness
		consistencySum += consistency
		accuracySum += accuracy

		if nullCount > 0 {
			issues = append(issues, Issue{
				Column: col.Name, Dimension: "completeness",
				Message:     fmt.Sprintf("%d missing values", nullCount),
				Severity:    severityFor(safeDiv(nullCount, total)),
				AffectedPct: safeDiv(nullCount, total) * 100,
			})
		}
		if invalid > 0 {
			issues = append(issues, Issue{
				Column: col.Name, Dimension: "validity",
				Message:     fmt.Sprintf("%d values don't match inferred type %s", invalid, col.Type),
				Severity:    severityFor(safeDiv(invalid, max1(len(values)))),
				AffectedPct: safeDiv(invalid, max1(len(values))) * 100,
			})
		}
		if col.Unique && len(distinct) < len(values) {
			dupCount := len(values) - len(distinct)
			issues = append(issues, Issue{
				Column: col.Name, Dimension: "uniqueness",
				Message:     fmt.Sprintf("%d duplicate values in a column marked unique", dupCount),
				Severity:    severityFor(safeDiv(dupCount, max1(len(values)))),
				AffectedPct: safeDiv(dupCount, max1(len(values))) * 100,
			})
		}
	}

	rep := Report{
		Completeness: completenessSum / n,
		Validity:     validitySum / n,
		Uniqueness:   uniquenessSum / n,
		Consistency:  consistencySum / n,
		Accuracy:     accuracySum / n,
		Issues:       issues,
	}
	rep.Overall = rep.Completeness*w.Completeness + rep.Validity*w.Validity +
		rep.Uniqueness*w.Uniqueness + rep.Consistency*w.Consistency + rep.Accuracy*w.Accuracy
	return rep
}

func severityFor(rate float64) string {
	switch {
	case rate >= 0.25:
		return "critical"
	case rate >= 0.05:
		return "warning"
	default:
		return "info"
	}
}

func typeMatches(s string, t schema.DataType) bool {
	switch t {
	case schema.TypeInteger:
		_, err := strconv.ParseInt(s, 10, 64)
		return err == nil
	case schema.TypeFloat:
		_, err := strconv.ParseFloat(s, 64)
		return err == nil
	case schema.TypeBoolean:
		_, err := strconv.ParseBool(s)
		return err == nil
	default:
		return true
	}
}

func patternOf(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteByte('9')
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			b.WriteByte('A')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func countConstraintViolations(values []string, c schema.Constraint) int {
	count := 0
	for _, v := range values {
		ok := true
		switch c.Kind {
		case "min":
			f, err := strconv.ParseFloat(v, 64)
			limit, _ := strconv.ParseFloat(c.Value, 64)
			ok = err == nil && f >= limit
		case "max":
			f, err := strconv.ParseFloat(v, 64)
			limit, _ := strconv.ParseFloat(c.Value, 64)
			ok = err == nil && f <= limit
		case "enum":
			ok = false
			for _, o := range strings.Split(c.Value, ",") {
				if strings.TrimSpace(o) == v {
					ok = true
					break
				}
			}
		}
		if !ok {
			count++
		}
	}
	return count
}

func toStr(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
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

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
