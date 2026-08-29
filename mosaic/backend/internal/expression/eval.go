package expression

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Env is the evaluation context: a single row's column values, exposed to
// Ident nodes. Compiled expressions are pure functions of Env.
type Env map[string]any

// Compiled wraps a parsed AST for fast repeated evaluation, e.g. once per
// row of a multi-million-row dataset inside Filter Rows / Generate Column.
type Compiled struct{ root Node }

// Compile parses src once; call Eval per row afterward.
func Compile(src string) (*Compiled, error) {
	root, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return &Compiled{root: root}, nil
}

// Eval evaluates the compiled expression against a single row environment.
func (c *Compiled) Eval(env Env) (any, error) {
	return eval(c.root, env)
}

// EvalBool is a convenience wrapper used by the Filter Rows node.
func (c *Compiled) EvalBool(env Env) (bool, error) {
	v, err := c.Eval(env)
	if err != nil {
		return false, err
	}
	return truthy(v), nil
}

func eval(n Node, env Env) (any, error) {
	switch t := n.(type) {
	case NumberLit:
		return t.Value, nil
	case StringLit:
		return t.Value, nil
	case Ident:
		if v, ok := env[t.Name]; ok {
			return v, nil
		}
		return nil, nil
	case Unary:
		v, err := eval(t.Expr, env)
		if err != nil {
			return nil, err
		}
		switch t.Op {
		case "!":
			return !truthy(v), nil
		case "-":
			return -toNumber(v), nil
		}
		return nil, fmt.Errorf("expression: unknown unary op %q", t.Op)
	case Binary:
		return evalBinary(t, env)
	case Call:
		args := make([]any, len(t.Args))
		for i, a := range t.Args {
			v, err := eval(a, env)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}
		return callFunction(t.Func, args)
	}
	return nil, fmt.Errorf("expression: unknown node type %T", n)
}

func evalBinary(b Binary, env Env) (any, error) {
	// Short-circuit boolean operators.
	if b.Op == "&&" {
		l, err := eval(b.Left, env)
		if err != nil || !truthy(l) {
			return false, err
		}
		r, err := eval(b.Right, env)
		return truthy(r), err
	}
	if b.Op == "||" {
		l, err := eval(b.Left, env)
		if err != nil {
			return nil, err
		}
		if truthy(l) {
			return true, nil
		}
		r, err := eval(b.Right, env)
		return truthy(r), err
	}

	l, err := eval(b.Left, env)
	if err != nil {
		return nil, err
	}
	r, err := eval(b.Right, env)
	if err != nil {
		return nil, err
	}

	switch b.Op {
	case "+":
		if ls, ok := l.(string); ok {
			return ls + toString(r), nil
		}
		if rs, ok := r.(string); ok {
			return toString(l) + rs, nil
		}
		return toNumber(l) + toNumber(r), nil
	case "-":
		return toNumber(l) - toNumber(r), nil
	case "*":
		return toNumber(l) * toNumber(r), nil
	case "/":
		rv := toNumber(r)
		if rv == 0 {
			return nil, fmt.Errorf("expression: division by zero")
		}
		return toNumber(l) / rv, nil
	case "%":
		rv := toNumber(r)
		if rv == 0 {
			return nil, fmt.Errorf("expression: modulo by zero")
		}
		return float64(int64(toNumber(l)) % int64(rv)), nil
	case "==":
		return looseEquals(l, r), nil
	case "!=":
		return !looseEquals(l, r), nil
	case ">":
		return compare(l, r) > 0, nil
	case "<":
		return compare(l, r) < 0, nil
	case ">=":
		return compare(l, r) >= 0, nil
	case "<=":
		return compare(l, r) <= 0, nil
	case "??":
		if l == nil {
			return r, nil
		}
		return l, nil
	}
	return nil, fmt.Errorf("expression: unknown operator %q", b.Op)
}

// ---- built-in function library --------------------------------------------

func callFunction(name string, args []any) (any, error) {
	switch strings.ToLower(name) {
	// -- string --
	case "contains":
		return strings.Contains(toString(arg(args, 0)), toString(arg(args, 1))), nil
	case "startswith":
		return strings.HasPrefix(toString(arg(args, 0)), toString(arg(args, 1))), nil
	case "endswith":
		return strings.HasSuffix(toString(arg(args, 0)), toString(arg(args, 1))), nil
	case "upper":
		return strings.ToUpper(toString(arg(args, 0))), nil
	case "lower":
		return strings.ToLower(toString(arg(args, 0))), nil
	case "trim":
		return strings.TrimSpace(toString(arg(args, 0))), nil
	case "concat":
		var b strings.Builder
		for _, a := range args {
			b.WriteString(toString(a))
		}
		return b.String(), nil
	case "length", "len":
		return float64(len([]rune(toString(arg(args, 0))))), nil
	case "replace":
		return strings.ReplaceAll(toString(arg(args, 0)), toString(arg(args, 1)), toString(arg(args, 2))), nil
	case "substr":
		s := []rune(toString(arg(args, 0)))
		start := int(toNumber(arg(args, 1)))
		end := len(s)
		if len(args) > 2 {
			end = int(toNumber(arg(args, 2)))
		}
		if start < 0 {
			start = 0
		}
		if end > len(s) {
			end = len(s)
		}
		if start > end {
			return "", nil
		}
		return string(s[start:end]), nil

	// -- numeric --
	case "round":
		digits := 0
		if len(args) > 1 {
			digits = int(toNumber(args[1]))
		}
		mult := pow10(digits)
		return roundf(toNumber(arg(args, 0))*mult) / mult, nil
	case "abs":
		v := toNumber(arg(args, 0))
		if v < 0 {
			return -v, nil
		}
		return v, nil
	case "min":
		m := toNumber(arg(args, 0))
		for _, a := range args[1:] {
			if v := toNumber(a); v < m {
				m = v
			}
		}
		return m, nil
	case "max":
		m := toNumber(arg(args, 0))
		for _, a := range args[1:] {
			if v := toNumber(a); v > m {
				m = v
			}
		}
		return m, nil

	// -- boolean / null handling --
	case "isnull":
		return arg(args, 0) == nil, nil
	case "isempty":
		v := arg(args, 0)
		return v == nil || toString(v) == "", nil
	case "coalesce":
		for _, a := range args {
			if a != nil && toString(a) != "" {
				return a, nil
			}
		}
		return nil, nil
	case "ifelse", "if":
		if truthy(arg(args, 0)) {
			return arg(args, 1), nil
		}
		return arg(args, 2), nil

	// -- date/time --
	case "now":
		return time.Now().Format(time.RFC3339), nil
	case "year", "month", "day":
		t, err := parseAnyDate(toString(arg(args, 0)))
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(name) {
		case "year":
			return float64(t.Year()), nil
		case "month":
			return float64(t.Month()), nil
		default:
			return float64(t.Day()), nil
		}
	}
	return nil, fmt.Errorf("expression: unknown function %q", name)
}

func arg(args []any, i int) any {
	if i < len(args) {
		return args[i]
	}
	return nil
}

func parseAnyDate(s string) (time.Time, error) {
	layouts := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05", "2006/01/02"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("expression: cannot parse date %q", s)
}

// ---- coercion helpers -------------------------------------------------

func truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t != "" && strings.ToLower(t) != "false"
	default:
		return true
	}
}

func toNumber(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case bool:
		if t {
			return 1
		}
		return 0
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		return 0
	}
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func looseEquals(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch a.(type) {
	case float64, int:
		return toNumber(a) == toNumber(b)
	}
	return toString(a) == toString(b)
}

func compare(a, b any) int {
	af, aok := numericLike(a)
	bf, bok := numericLike(b)
	if aok && bok {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	}
	as, bs := toString(a), toString(b)
	return strings.Compare(as, bs)
}

func numericLike(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	}
	return 0, false
}

func pow10(n int) float64 {
	p := 1.0
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}

func roundf(f float64) float64 {
	if f < 0 {
		return -roundf(-f)
	}
	i := int64(f)
	if f-float64(i) >= 0.5 {
		i++
	}
	return float64(i)
}
