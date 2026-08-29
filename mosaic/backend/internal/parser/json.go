package parser

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"mosaic/internal/schema"
)

func init() {
	Register(&jsonParser{})
	Register(&ndjsonParser{})
}

// ---- JSON (single document, array-of-objects or object-of-arrays) --------

type jsonParser struct{}

func (jsonParser) Name() string { return "json" }

func (jsonParser) Sniff(filename string, head []byte) float64 {
	lower := strings.ToLower(filename)
	trimmed := bytes.TrimSpace(head)
	if strings.HasSuffix(lower, ".json") {
		return 0.95
	}
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return 0.5
	}
	return 0
}

func (jsonParser) Parse(r io.Reader, opt Options) (Result, error) {
	res := Result{Format: "json"}
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return res, err
	}
	rows, columns := flattenToRows(raw)
	if opt.SampleLimit > 0 && len(rows) > opt.SampleLimit {
		rows = rows[:opt.SampleLimit]
	}
	res.Rows, res.Columns = rows, columns
	return res, nil
}

func (p jsonParser) Stream(r io.Reader, opt Options, handle RowHandler) error {
	res, err := p.Parse(r, Options{})
	if err != nil {
		return err
	}
	for _, row := range res.Rows {
		if err := handle(row); err != nil {
			return err
		}
	}
	return nil
}

// flattenToRows normalizes JSON documents of shape []object, {data:[...]}
// or a single object into MOSAIC's tabular Row representation. Nested
// values are preserved as-is (type "json") for the JSON Visual Explorer /
// Flatten JSON node to process explicitly rather than silently losing data.
func flattenToRows(raw any) ([]schema.Row, []string) {
	var items []any
	switch v := raw.(type) {
	case []any:
		items = v
	case map[string]any:
		// Common API envelope shapes: find the first array value.
		found := false
		for _, key := range []string{"data", "items", "results", "rows", "records"} {
			if arr, ok := v[key].([]any); ok {
				items, found = arr, true
				break
			}
		}
		if !found {
			items = []any{v}
		}
	default:
		items = []any{v}
	}

	colSet := map[string]bool{}
	colOrder := []string{}
	rows := make([]schema.Row, 0, len(items))
	for _, it := range items {
		obj, ok := it.(map[string]any)
		row := schema.Row{}
		if ok {
			for k, val := range obj {
				row[k] = val
				if !colSet[k] {
					colSet[k] = true
					colOrder = append(colOrder, k)
				}
			}
		} else {
			row["value"] = it
			if !colSet["value"] {
				colSet["value"] = true
				colOrder = append(colOrder, "value")
			}
		}
		rows = append(rows, row)
	}
	return rows, colOrder
}

// ---- NDJSON / JSONL (one JSON object per line) ----------------------------

type ndjsonParser struct{}

func (ndjsonParser) Name() string { return "ndjson" }

func (ndjsonParser) Sniff(filename string, head []byte) float64 {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".ndjson") || strings.HasSuffix(lower, ".jsonl") {
		return 0.95
	}
	trimmed := bytes.TrimSpace(head)
	if bytes.Count(trimmed, []byte("\n")) > 0 && len(trimmed) > 0 && trimmed[0] == '{' {
		return 0.35
	}
	return 0
}

func (ndjsonParser) Parse(r io.Reader, opt Options) (Result, error) {
	res := Result{Format: "ndjson"}
	colSet := map[string]bool{}
	err := (ndjsonParser{}).Stream(r, opt, func(row schema.Row) error {
		for k := range row {
			if !colSet[k] {
				colSet[k] = true
				res.Columns = append(res.Columns, k)
			}
		}
		res.Rows = append(res.Rows, row)
		if opt.SampleLimit > 0 && len(res.Rows) >= opt.SampleLimit {
			return errStop
		}
		return nil
	})
	if err == errStop {
		err = nil
	}
	return res, err
}

var errStop = stopErr{}

type stopErr struct{}

func (stopErr) Error() string { return "stop" }

func (ndjsonParser) Stream(r io.Reader, opt Options, handle RowHandler) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			continue // tolerate malformed lines, matching the Error Recovery philosophy
		}
		if err := handle(schema.Row(obj)); err != nil {
			return err
		}
	}
	return scanner.Err()
}
