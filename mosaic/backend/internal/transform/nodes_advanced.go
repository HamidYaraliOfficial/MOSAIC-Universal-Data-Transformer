package transform

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"mosaic/internal/schema"
)

func init() {
	Register("groupByAggregate", func() Node { return &GroupByAggregate{} })
	Register("join", func() Node { return &JoinNode{} })
	Register("union", func() Node { return &UnionNode{} })
	Register("pivot", func() Node { return &PivotNode{} })
	Register("unpivot", func() Node { return &UnpivotNode{} })
	Register("flattenJSON", func() Node { return &FlattenJSON{} })
	Register("sampling", func() Node { return &SamplingNode{} })
	Register("validateSchema", func() Node { return &ValidateSchema{} })
	Register("lookupEnrich", func() Node { return &LookupEnrich{} })
}

// ---- Group By / Aggregate ------------------------------------------------

type aggSpec struct {
	Column string `json:"column"`
	Func   string `json:"func"` // sum|avg|count|min|max|first|last
	As     string `json:"as"`
}

type GroupByAggregate struct{}

func (GroupByAggregate) Type() string { return "groupByAggregate" }

func (GroupByAggregate) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	keys := cfgStringSlice(ctx.Config, "keys")
	aggsAny, _ := ctx.Config["aggregations"].([]any)
	aggs := make([]aggSpec, 0, len(aggsAny))
	for _, a := range aggsAny {
		m, ok := a.(map[string]any)
		if !ok {
			continue
		}
		col, _ := m["column"].(string)
		fn, _ := m["func"].(string)
		as, _ := m["as"].(string)
		if as == "" {
			as = fn + "_" + col
		}
		aggs = append(aggs, aggSpec{Column: col, Func: fn, As: as})
	}

	type bucket struct {
		keyVals map[string]any
		rows    []schema.Row
	}
	groups := map[string]*bucket{}
	order := []string{}

	for _, r := range rows {
		var sig strings.Builder
		kv := make(map[string]any, len(keys))
		for _, k := range keys {
			v := r[k]
			kv[k] = v
			sig.WriteString(fmt.Sprint(v))
			sig.WriteByte('\x1f')
		}
		s := sig.String()
		b, ok := groups[s]
		if !ok {
			b = &bucket{keyVals: kv}
			groups[s] = b
			order = append(order, s)
		}
		b.rows = append(b.rows, r)
	}

	out := make([]schema.Row, 0, len(groups))
	for _, s := range order {
		b := groups[s]
		nr := make(schema.Row, len(keys)+len(aggs))
		for k, v := range b.keyVals {
			nr[k] = v
		}
		for _, a := range aggs {
			nr[a.As] = aggregate(b.rows, a)
		}
		out = append(out, nr)
	}
	return out, nil
}

func aggregate(rows []schema.Row, a aggSpec) any {
	switch a.Func {
	case "count":
		return float64(len(rows))
	case "first":
		if len(rows) > 0 {
			return rows[0][a.Column]
		}
		return nil
	case "last":
		if len(rows) > 0 {
			return rows[len(rows)-1][a.Column]
		}
		return nil
	}
	var sum float64
	n := 0
	mn, mx := 0.0, 0.0
	first := true
	for _, r := range rows {
		f, err := strconv.ParseFloat(fmt.Sprint(r[a.Column]), 64)
		if err != nil {
			continue
		}
		sum += f
		n++
		if first || f < mn {
			mn = f
		}
		if first || f > mx {
			mx = f
		}
		first = false
	}
	switch a.Func {
	case "sum":
		return sum
	case "avg":
		if n == 0 {
			return 0.0
		}
		return sum / float64(n)
	case "min":
		return mn
	case "max":
		return mx
	default:
		return nil
	}
}

// ---- Join (Inner / Left / Right / Full / Cross, multi-key) --------------

type JoinNode struct{}

func (JoinNode) Type() string { return "join" }

func (JoinNode) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	right := ctx.Inputs["right"]
	joinType := cfgString(ctx.Config, "joinType", "inner") // inner|left|right|full|cross
	leftKeys := cfgStringSlice(ctx.Config, "leftKeys")
	rightKeys := cfgStringSlice(ctx.Config, "rightKeys")
	prefix := cfgString(ctx.Config, "rightPrefix", "right_")

	if joinType == "cross" {
		out := make([]schema.Row, 0, len(rows)*len(right))
		for _, l := range rows {
			for _, r := range right {
				out = append(out, mergeRows(l, r, prefix))
			}
		}
		return out, nil
	}

	index := map[string][]schema.Row{}
	for _, r := range right {
		index[joinSig(r, rightKeys)] = append(index[joinSig(r, rightKeys)], r)
	}

	matchedRight := map[int]bool{}
	rightPos := map[string][]int{}
	for i, r := range right {
		s := joinSig(r, rightKeys)
		rightPos[s] = append(rightPos[s], i)
	}

	out := make([]schema.Row, 0, len(rows))
	for _, l := range rows {
		sig := joinSig(l, leftKeys)
		matches := index[sig]
		if len(matches) == 0 {
			if joinType == "inner" || joinType == "right" {
				continue
			}
			out = append(out, mergeRows(l, nil, prefix))
			continue
		}
		for _, idx := range rightPos[sig] {
			matchedRight[idx] = true
		}
		for _, r := range matches {
			out = append(out, mergeRows(l, r, prefix))
		}
	}

	if joinType == "right" || joinType == "full" {
		for i, r := range right {
			if matchedRight[i] {
				continue
			}
			out = append(out, mergeRows(nil, r, prefix))
		}
	}
	return out, nil
}

func joinSig(r schema.Row, keys []string) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(fmt.Sprint(r[k]))
		b.WriteByte('\x1f')
	}
	return b.String()
}

func mergeRows(l, r schema.Row, prefix string) schema.Row {
	out := schema.Row{}
	for k, v := range l {
		out[k] = v
	}
	for k, v := range r {
		key := k
		if _, exists := out[key]; exists {
			key = prefix + k
		}
		out[key] = v
	}
	return out
}

// ---- Union ---------------------------------------------------------------

type UnionNode struct{}

func (UnionNode) Type() string { return "union" }

func (UnionNode) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	dedupe := cfgBool(ctx.Config, "distinct", false)
	other := ctx.Inputs["b"]
	combined := append(append([]schema.Row(nil), rows...), other...)
	if !dedupe {
		return combined, nil
	}
	seen := map[string]bool{}
	out := make([]schema.Row, 0, len(combined))
	for _, r := range combined {
		sig := fmt.Sprint(r)
		if seen[sig] {
			continue
		}
		seen[sig] = true
		out = append(out, r)
	}
	return out, nil
}

// ---- Pivot / Unpivot -----------------------------------------------------

type PivotNode struct{}

func (PivotNode) Type() string { return "pivot" }

func (PivotNode) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	indexCol := cfgString(ctx.Config, "indexColumn", "")
	pivotCol := cfgString(ctx.Config, "pivotColumn", "")
	valueCol := cfgString(ctx.Config, "valueColumn", "")
	aggFunc := cfgString(ctx.Config, "aggFunc", "first")
	if indexCol == "" || pivotCol == "" || valueCol == "" {
		return rows, nil
	}

	type cell struct{ vals []schema.Row }
	groups := map[string]schema.Row{}
	order := []string{}
	cellData := map[string]map[string]*cell{}

	for _, r := range rows {
		idx := fmt.Sprint(r[indexCol])
		if _, ok := groups[idx]; !ok {
			groups[idx] = schema.Row{indexCol: r[indexCol]}
			order = append(order, idx)
			cellData[idx] = map[string]*cell{}
		}
		col := fmt.Sprint(r[pivotCol])
		c, ok := cellData[idx][col]
		if !ok {
			c = &cell{}
			cellData[idx][col] = c
		}
		c.vals = append(c.vals, r)
	}

	for idx, cols := range cellData {
		for col, c := range cols {
			groups[idx][col] = aggregate(c.vals, aggSpec{Column: valueCol, Func: aggFunc})
		}
	}

	out := make([]schema.Row, 0, len(order))
	for _, idx := range order {
		out = append(out, groups[idx])
	}
	return out, nil
}

type UnpivotNode struct{}

func (UnpivotNode) Type() string { return "unpivot" }

func (UnpivotNode) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	idCols := cfgStringSlice(ctx.Config, "idColumns")
	valueCols := cfgStringSlice(ctx.Config, "valueColumns")
	nameField := cfgString(ctx.Config, "nameField", "variable")
	valueField := cfgString(ctx.Config, "valueField", "value")

	out := make([]schema.Row, 0, len(rows)*len(valueCols))
	for _, r := range rows {
		for _, vc := range valueCols {
			nr := schema.Row{}
			for _, id := range idCols {
				nr[id] = r[id]
			}
			nr[nameField] = vc
			nr[valueField] = r[vc]
			out = append(out, nr)
		}
	}
	return out, nil
}

// ---- Flatten JSON --------------------------------------------------------

type FlattenJSON struct{}

func (FlattenJSON) Type() string { return "flattenJSON" }

func (FlattenJSON) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	col := cfgString(ctx.Config, "column", "")
	sep := cfgString(ctx.Config, "separator", ".")
	if col == "" {
		return rows, nil
	}
	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := r.Clone()
		if obj, ok := r[col].(map[string]any); ok {
			delete(nr, col)
			flatten(obj, col, sep, nr)
		}
		out[i] = nr
	}
	return out, nil
}

func flatten(obj map[string]any, prefix, sep string, into schema.Row) {
	for k, v := range obj {
		key := prefix + sep + k
		if nested, ok := v.(map[string]any); ok {
			flatten(nested, key, sep, into)
			continue
		}
		into[key] = v
	}
}

// ---- Sampling --------------------------------------------------------

type SamplingNode struct{}

func (SamplingNode) Type() string { return "sampling" }

func (SamplingNode) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	method := cfgString(ctx.Config, "method", "random") // random|first|systematic
	size := 0
	if v, ok := ctx.Config["size"].(float64); ok {
		size = int(v)
	}
	if size <= 0 || size >= len(rows) {
		return rows, nil
	}
	switch method {
	case "first":
		return rows[:size], nil
	case "systematic":
		step := len(rows) / size
		if step < 1 {
			step = 1
		}
		out := make([]schema.Row, 0, size)
		for i := 0; i < len(rows) && len(out) < size; i += step {
			out = append(out, rows[i])
		}
		return out, nil
	default: // random reservoir sampling
		idx := rand.Perm(len(rows))[:size]
		sort.Ints(idx)
		out := make([]schema.Row, size)
		for i, ix := range idx {
			out[i] = rows[ix]
		}
		return out, nil
	}
}

// ---- Validate Schema ---------------------------------------------------

type ValidateSchema struct{}

func (ValidateSchema) Type() string { return "validateSchema" }

func (ValidateSchema) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	rulesAny, _ := ctx.Config["rules"].([]any)
	out := make([]schema.Row, 0, len(rows))
	for _, r := range rows {
		valid := true
		for _, ruleAny := range rulesAny {
			rule, ok := ruleAny.(map[string]any)
			if !ok {
				continue
			}
			col, _ := rule["column"].(string)
			kind, _ := rule["kind"].(string)
			value, _ := rule["value"].(string)
			if !validateRule(r[col], kind, value) {
				valid = false
				ctx.recordError(r, fmt.Errorf("validateSchema: column %q failed rule %q", col, kind))
				break
			}
		}
		if valid {
			out = append(out, r)
		} else if ctx.OnError == OnErrorStop {
			return nil, fmt.Errorf("validateSchema: row failed validation")
		}
	}
	return out, nil
}

func validateRule(v any, kind, value string) bool {
	switch kind {
	case "notNull":
		return v != nil && fmt.Sprint(v) != ""
	case "min":
		f, err := strconv.ParseFloat(fmt.Sprint(v), 64)
		limit, _ := strconv.ParseFloat(value, 64)
		return err == nil && f >= limit
	case "max":
		f, err := strconv.ParseFloat(fmt.Sprint(v), 64)
		limit, _ := strconv.ParseFloat(value, 64)
		return err == nil && f <= limit
	case "enum":
		options := strings.Split(value, ",")
		for _, o := range options {
			if strings.TrimSpace(o) == fmt.Sprint(v) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

// ---- Lookup / Enrich (single-key left join against a reference table) ---

type LookupEnrich struct{}

func (LookupEnrich) Type() string { return "lookupEnrich" }

func (LookupEnrich) Run(ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	ref := ctx.Inputs["reference"]
	key := cfgString(ctx.Config, "key", "")
	refKey := cfgString(ctx.Config, "referenceKey", key)
	fields := cfgStringSlice(ctx.Config, "fields")

	index := map[string]schema.Row{}
	for _, r := range ref {
		index[fmt.Sprint(r[refKey])] = r
	}

	out := make([]schema.Row, len(rows))
	for i, r := range rows {
		nr := r.Clone()
		if match, ok := index[fmt.Sprint(r[key])]; ok {
			for _, f := range fields {
				nr[f] = match[f]
			}
		}
		out[i] = nr
	}
	return out, nil
}
