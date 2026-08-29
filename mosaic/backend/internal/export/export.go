// Package export implements the Export Studio's format writers. Every
// writer takes the same (columns, rows, Options) shape so the UI can swap
// target formats without touching pipeline logic.
package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"mosaic/internal/schema"
)

// Options configures a single export run: delimiter, encoding, naming and
// (for SQL) target table name — the same fields the Export Studio panel
// exposes to the user.
type Options struct {
	Delimiter  rune   `json:"delimiter,omitempty"`
	TableName  string `json:"tableName,omitempty"`
	Pretty     bool   `json:"pretty"`
	NullString string `json:"nullString,omitempty"`
}

// Writer is implemented by every export format.
type Writer func(w io.Writer, columns []string, rows []schema.Row, opt Options) error

// Registry of built-in writers, keyed by format name shown in Export Studio.
var Registry = map[string]Writer{
	"json":     WriteJSON,
	"csv":      WriteCSV,
	"xml":      WriteXML,
	"yaml":     WriteYAML,
	"markdown": WriteMarkdown,
	"sql":      WriteSQLInsert,
}

// WriteJSON emits an array-of-objects document.
func WriteJSON(w io.Writer, columns []string, rows []schema.Row, opt Options) error {
	enc := json.NewEncoder(w)
	if opt.Pretty {
		enc.SetIndent("", "  ")
	}
	out := make([]schema.Row, len(rows))
	copy(out, rows)
	return enc.Encode(out)
}

// WriteCSV emits delimited text with a header row.
func WriteCSV(w io.Writer, columns []string, rows []schema.Row, opt Options) error {
	delim := opt.Delimiter
	if delim == 0 {
		delim = ','
	}
	cw := csv.NewWriter(w)
	cw.Comma = delim
	if err := cw.Write(columns); err != nil {
		return err
	}
	for _, r := range rows {
		record := make([]string, len(columns))
		for i, c := range columns {
			record[i] = valueToString(r[c], opt)
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteXML emits a simple <rows><row><col>val</col></row></rows> document.
func WriteXML(w io.Writer, columns []string, rows []schema.Row, opt Options) error {
	if _, err := fmt.Fprintln(w, "<rows>"); err != nil {
		return err
	}
	for _, r := range rows {
		fmt.Fprintln(w, "  <row>")
		for _, c := range columns {
			fmt.Fprintf(w, "    <%s>%s</%s>\n", xmlSafe(c), xmlEscape(valueToString(r[c], opt)), xmlSafe(c))
		}
		fmt.Fprintln(w, "  </row>")
	}
	_, err := fmt.Fprintln(w, "</rows>")
	return err
}

func xmlSafe(s string) string {
	return strings.NewReplacer(" ", "_", "<", "_", ">", "_").Replace(s)
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}

// WriteYAML emits a `- col: value` list document (hand-rolled to avoid an
// external dependency; sufficient for MOSAIC's flat tabular export case).
func WriteYAML(w io.Writer, columns []string, rows []schema.Row, opt Options) error {
	for _, r := range rows {
		if _, err := fmt.Fprintln(w, "-"); err != nil {
			return err
		}
		for _, c := range columns {
			v := valueToString(r[c], opt)
			if _, err := fmt.Fprintf(w, "  %s: %s\n", c, yamlScalar(v)); err != nil {
				return err
			}
		}
	}
	return nil
}

func yamlScalar(v string) string {
	if v == "" {
		return "null"
	}
	if strings.ContainsAny(v, ":#{}[]&*!|>'\"%@`") || strings.TrimSpace(v) != v {
		return strconv.Quote(v)
	}
	return v
}

// WriteMarkdown emits a GitHub-flavored Markdown table.
func WriteMarkdown(w io.Writer, columns []string, rows []schema.Row, opt Options) error {
	fmt.Fprintf(w, "| %s |\n", strings.Join(columns, " | "))
	sep := make([]string, len(columns))
	for i := range sep {
		sep[i] = "---"
	}
	fmt.Fprintf(w, "| %s |\n", strings.Join(sep, " | "))
	for _, r := range rows {
		vals := make([]string, len(columns))
		for i, c := range columns {
			vals[i] = strings.ReplaceAll(valueToString(r[c], opt), "|", "\\|")
		}
		fmt.Fprintf(w, "| %s |\n", strings.Join(vals, " | "))
	}
	return nil
}

// WriteSQLInsert emits standard multi-row INSERT statements, batched to
// keep individual statements a reasonable size for most SQL engines.
func WriteSQLInsert(w io.Writer, columns []string, rows []schema.Row, opt Options) error {
	table := opt.TableName
	if table == "" {
		table = "mosaic_export"
	}
	quotedCols := make([]string, len(columns))
	for i, c := range columns {
		quotedCols[i] = `"` + c + `"`
	}
	const batchSize = 500
	for start := 0; start < len(rows); start += batchSize {
		end := start + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		fmt.Fprintf(w, "INSERT INTO %s (%s) VALUES\n", table, strings.Join(quotedCols, ", "))
		for i, r := range rows[start:end] {
			vals := make([]string, len(columns))
			for j, c := range columns {
				vals[j] = sqlLiteral(r[c])
			}
			sep := ","
			if i == end-start-1 {
				sep = ";"
			}
			fmt.Fprintf(w, "  (%s)%s\n", strings.Join(vals, ", "), sep)
		}
	}
	return nil
}

func sqlLiteral(v any) string {
	if v == nil {
		return "NULL"
	}
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	default:
		s := fmt.Sprint(v)
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
}

func valueToString(v any, opt Options) string {
	if v == nil {
		return opt.NullString
	}
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// FormatNames returns every registered export format, sorted, for the
// Export Studio's format picker.
func FormatNames() []string {
	names := make([]string, 0, len(Registry))
	for n := range Registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
