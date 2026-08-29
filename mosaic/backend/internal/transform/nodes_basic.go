package transform

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"mosaic/internal/expression"
	"mosaic/internal/schema"
)

func init() {
	Register("selectColumns", func() Node { return &SelectColumns{} })
	Register("renameColumns", func() Node { return &RenameColumns{} })
	Register("filterRows", func() Node { return &FilterRows{} })
	Register("sort", func() Node { return &SortNode{} })
	Register("deduplicate", func() Node { return &Deduplicate{} })
	Register("typeCast", func() Node { return &TypeCast{} })
	Register("fillMissingValues", func() Node { return &FillMissing{} })
	Register("generateColumn", func() Node { return &GenerateColumn{} })
	Register("regexExtract", func() Node { return &RegexExtract{} })
	Register("splitColumn", func() Node { return &SplitColumn{} })
	Register("mergeColumn", func() Node { return &MergeColumn{} })
	Register("stringTransform", func() Node { return &StringTransform{} })
	Register("mapValues", func() Node { return &MapValues{} })
}

func cfgString(c map[string]any, key, def string) string {
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func cfgStringSlice(c map[string]any, key string) []string {
	v, ok := c[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, a := range arr {
		if s, ok := a.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func cfgBool(c map[string]any, key string, def bool) bool {
	if v, ok := c[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

// ---- Select Columns --------------------------------------------------

type SelectColumns struct{}

func (SelectColumns) Type() string { return "selectColumns" }

func (SelectColumns) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	cols := cfgStringSlice(ctx.Config, "columns")
	if len(cols) == 0 {
		return rows, nil
	}
	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := make(schema.Row, len(cols))
		for _, c := range cols {
			nr[c] = r[c]
		}
		out[i] = nr
	}
	return out, nil
}

// ---- Rename Columns ----------------------------------------------------

type RenameColumns struct{}

func (RenameColumns) Type() string { return "renameColumns" }

func (RenameColumns) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	mapping, _ := ctx.Config["mapping"].(map[string]any)
	if len(mapping) == 0 {
		return rows, nil
	}
	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := r.Clone()
		for from, toAny := range mapping {
			to, ok := toAny.(string)
			if !ok {
				continue
			}
			if v, exists := nr[from]; exists {
				delete(nr, from)
				nr[to] = v
			}
		}
		out[i] = nr
	}
	return out, nil
}

// ---- Filter Rows (Expression Engine powered) ----------------------------

type FilterRows struct{}

func (FilterRows) Type() string { return "filterRows" }

func (FilterRows) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	src := cfgString(ctx.Config, "expression", "")
	if src == "" {
		return rows, nil
	}
	compiled, err := expression.Compile(src)
	if err != nil {
		return nil, fmt.Errorf("filterRows: %w", err)
	}
	out := make([]schema.Row, 0, len(rows))
	for _, r := range rows {
		keep, err := compiled.EvalBool(expression.Env(r))
		if err != nil {
			if ctx.OnError == OnErrorStop {
				return nil, err
			}
			ctx.recordError(r, err)
			continue
		}
		if keep {
			out = append(out, r)
		}
	}
	return out, nil
}

// ---- Sort ----------------------------------------------------------------

type SortNode struct{}

func (SortNode) Type() string { return "sort" }

type sortKey struct {
	Column string `json:"column"`
	Desc   bool   `json:"desc"`
}

func (SortNode) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	keysAny, _ := ctx.Config["keys"].([]any)
	if len(keysAny) == 0 {
		return rows, nil
	}
	keys := make([]sortKey, 0, len(keysAny))
	for _, k := range keysAny {
		m, ok := k.(map[string]any)
		if !ok {
			continue
		}
		col, _ := m["column"].(string)
		desc, _ := m["desc"].(bool)
		keys = append(keys, sortKey{Column: col, Desc: desc})
	}
	out := append([]schema.Row(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool {
		for _, k := range keys {
			c := compareValues(out[i][k.Column], out[j][k.Column])
			if c == 0 {
				continue
			}
			if k.Desc {
				return c > 0
			}
			return c < 0
		}
		return false
	})
	return out, nil
}

func compareValues(a, b any) int {
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		af, aerr := strconv.ParseFloat(as, 64)
		bf, berr := strconv.ParseFloat(bs, 64)
		if aerr == nil && berr == nil {
			switch {
			case af < bf:
				return -1
			case af > bf:
				return 1
			default:
				return 0
			}
		}
		return strings.Compare(as, bs)
	}
	return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
}

// ---- Deduplicate -----------------------------------------------------

type Deduplicate struct{}

func (Deduplicate) Type() string { return "deduplicate" }

func (Deduplicate) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	keys := cfgStringSlice(ctx.Config, "keys")
	seen := make(map[string]bool, len(rows))
	out := make([]schema.Row, 0, len(rows))
	for _, r := range rows {
		var sig string
		if len(keys) == 0 {
			sig = fmt.Sprint(r)
		} else {
			var b strings.Builder
			for _, k := range keys {
				b.WriteString(fmt.Sprint(r[k]))
				b.WriteByte('\x1f')
			}
			sig = b.String()
		}
		if seen[sig] {
			continue
		}
		seen[sig] = true
		out = append(out, r)
	}
	return out, nil
}

// ---- Type Cast -------------------------------------------------------

type TypeCast struct{}

func (TypeCast) Type() string { return "typeCast" }

func (TypeCast) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	casts, _ := ctx.Config["casts"].(map[string]any)
	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := r.Clone()
		for col, typAny := range casts {
			typ, _ := typAny.(string)
			v, err := castValue(nr[col], schema.DataType(typ))
			if err != nil {
				if ctx.OnError == OnErrorStop {
					return nil, err
				}
				ctx.recordError(r, err)
				continue
			}
			nr[col] = v
		}
		out[i] = nr
	}
	return out, nil
}

func castValue(v any, t schema.DataType) (any, error) {
	if v == nil {
		return nil, nil
	}
	s := fmt.Sprint(v)
	switch t {
	case schema.TypeInteger:
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return nil, fmt.Errorf("typeCast: cannot cast %q to integer", s)
		}
		return float64(int64(f)), nil
	case schema.TypeFloat:
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return nil, fmt.Errorf("typeCast: cannot cast %q to float", s)
		}
		return f, nil
	case schema.TypeBoolean:
		b, err := strconv.ParseBool(strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("typeCast: cannot cast %q to boolean", s)
		}
		return b, nil
	case schema.TypeString:
		return s, nil
	default:
		return v, nil
	}
}

// ---- Fill Missing Values -----------------------------------------------

type FillMissing struct{}

func (FillMissing) Type() string { return "fillMissingValues" }

func (FillMissing) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	fills, _ := ctx.Config["fills"].(map[string]any)
	strategy := cfgString(ctx.Config, "strategy", "static") // static | mean | mode
	out := make([]schema.Row, len(rows))

	means := map[string]float64{}
	modes := map[string]any{}
	if strategy == "mean" || strategy == "mode" {
		means, modes = computeFillStats(rows, fills)
	}

	for i, r := range rows {
		nr := r.Clone()
		for col := range fills {
			if v, ok := nr[col]; ok && v != nil && fmt.Sprint(v) != "" {
				continue
			}
			switch strategy {
			case "mean":
				nr[col] = means[col]
			case "mode":
				nr[col] = modes[col]
			default:
				nr[col] = fills[col]
			}
		}
		out[i] = nr
	}
	return out, nil
}

func computeFillStats(rows []schema.Row, fills map[string]any) (map[string]float64, map[string]any) {
	sums := map[string]float64{}
	counts := map[string]int{}
	freq := map[string]map[string]int{}
	for col := range fills {
		freq[col] = map[string]int{}
	}
	for _, r := range rows {
		for col := range fills {
			v := r[col]
			if v == nil {
				continue
			}
			s := fmt.Sprint(v)
			if s == "" {
				continue
			}
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				sums[col] += f
				counts[col]++
			}
			freq[col][s]++
		}
	}
	means := map[string]float64{}
	for col, s := range sums {
		if counts[col] > 0 {
			means[col] = s / float64(counts[col])
		}
	}
	modes := map[string]any{}
	for col, f := range freq {
		best, bestCount := "", -1
		for v, c := range f {
			if c > bestCount {
				best, bestCount = v, c
			}
		}
		modes[col] = best
	}
	return means, modes
}

// ---- Generate Column (Expression Engine powered) ------------------------

type GenerateColumn struct{}

func (GenerateColumn) Type() string { return "generateColumn" }

func (GenerateColumn) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	col := cfgString(ctx.Config, "column", "")
	src := cfgString(ctx.Config, "expression", "")
	if col == "" || src == "" {
		return rows, nil
	}
	compiled, err := expression.Compile(src)
	if err != nil {
		return nil, fmt.Errorf("generateColumn: %w", err)
	}
	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := r.Clone()
		v, err := compiled.Eval(expression.Env(r))
		if err != nil {
			if ctx.OnError == OnErrorStop {
				return nil, err
			}
			ctx.recordError(r, err)
			out[i] = nr
			continue
		}
		nr[col] = v
		out[i] = nr
	}
	return out, nil
}

// ---- Regex Extract ---------------------------------------------------

type RegexExtract struct{}

func (RegexExtract) Type() string { return "regexExtract" }

func (RegexExtract) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	src := cfgString(ctx.Config, "column", "")
	pattern := cfgString(ctx.Config, "pattern", "")
	target := cfgString(ctx.Config, "targetColumn", src+"_extracted")
	if src == "" || pattern == "" {
		return rows, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("regexExtract: invalid pattern: %w", err)
	}
	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := r.Clone()
		match := re.FindStringSubmatch(fmt.Sprint(r[src]))
		if len(match) > 1 {
			nr[target] = match[1]
		} else if len(match) == 1 {
			nr[target] = match[0]
		} else {
			nr[target] = nil
		}
		out[i] = nr
	}
	return out, nil
}

// ---- Split Column ------------------------------------------------------

type SplitColumn struct{}

func (SplitColumn) Type() string { return "splitColumn" }

func (SplitColumn) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	src := cfgString(ctx.Config, "column", "")
	delim := cfgString(ctx.Config, "delimiter", ",")
	targets := cfgStringSlice(ctx.Config, "targetColumns")
	if src == "" || len(targets) == 0 {
		return rows, nil
	}
	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := r.Clone()
		parts := strings.Split(fmt.Sprint(r[src]), delim)
		for j, t := range targets {
			if j < len(parts) {
				nr[t] = strings.TrimSpace(parts[j])
			} else {
				nr[t] = nil
			}
		}
		out[i] = nr
	}
	return out, nil
}

// ---- Merge Column ------------------------------------------------------

type MergeColumn struct{}

func (MergeColumn) Type() string { return "mergeColumn" }

func (MergeColumn) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	sources := cfgStringSlice(ctx.Config, "columns")
	target := cfgString(ctx.Config, "targetColumn", "merged")
	sep := cfgString(ctx.Config, "separator", " ")
	if len(sources) == 0 {
		return rows, nil
	}
	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := r.Clone()
		parts := make([]string, 0, len(sources))
		for _, s := range sources {
			if v := r[s]; v != nil {
				parts = append(parts, fmt.Sprint(v))
			}
		}
		nr[target] = strings.Join(parts, sep)
		out[i] = nr
	}
	return out, nil
}

// ---- String Transform (case, trim, pad) ---------------------------------

type StringTransform struct{}

func (StringTransform) Type() string { return "stringTransform" }

func (StringTransform) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	col := cfgString(ctx.Config, "column", "")
	op := cfgString(ctx.Config, "operation", "trim") // upper|lower|trim|title|capitalize
	if col == "" {
		return rows, nil
	}
	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := r.Clone()
		s := fmt.Sprint(r[col])
		switch op {
		case "upper":
			nr[col] = strings.ToUpper(s)
		case "lower":
			nr[col] = strings.ToLower(s)
		case "trim":
			nr[col] = strings.TrimSpace(s)
		case "title":
			nr[col] = strings.Title(strings.ToLower(s))
		case "capitalize":
			if len(s) > 0 {
				nr[col] = strings.ToUpper(s[:1]) + s[1:]
			}
		}
		out[i] = nr
	}
	return out, nil
}

// ---- Map Values (value lookup / recode) ---------------------------------

type MapValues struct{}

func (MapValues) Type() string { return "mapValues" }

func (MapValues) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	col := cfgString(ctx.Config, "column", "")
	mapping, _ := ctx.Config["mapping"].(map[string]any)
	def := ctx.Config["default"]
	if col == "" {
		return rows, nil
	}
	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := r.Clone()
		key := fmt.Sprint(r[col])
		if v, ok := mapping[key]; ok {
			nr[col] = v
		} else if def != nil {
			nr[col] = def
		}
		out[i] = nr
	}
	return out, nil
}
