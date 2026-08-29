package expression

import "fmt"

// Node is the AST for a parsed expression. It's small and closed by design
// so the Go evaluator can exhaustively switch on it without reflection.
type Node interface{ isNode() }

type NumberLit struct{ Value float64 }
type StringLit struct{ Value string }
type Ident struct{ Name string } // column reference, e.g. `age`
type Unary struct {
	Op   string
	Expr Node
}
type Binary struct {
	Op          string
	Left, Right Node
}
type Call struct {
	Func string
	Args []Node
}

func (NumberLit) isNode() {}
func (StringLit) isNode() {}
func (Ident) isNode()     {}
func (Unary) isNode()     {}
func (Binary) isNode()    {}
func (Call) isNode()      {}

// precedence table, low to high; expressions like `a && b == c * d` parse
// exactly as a spreadsheet or SQL WHERE clause user would expect.
var precedence = map[string]int{
	"||": 1, "??": 1,
	"&&": 2,
	"==": 3, "!=": 3, ">": 3, "<": 3, ">=": 3, "<=": 3,
	"+": 4, "-": 4,
	"*": 5, "/": 5, "%": 5,
}

type parser struct {
	toks []token
	pos  int
}

// Parse compiles expression source into an AST, ready for repeated
// evaluation across many rows (compile once, evaluate millions of times —
// this is what lets Filter Rows / Generate Column run fast in Go).
func Parse(src string) (Node, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	node, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokEOF {
		return nil, fmt.Errorf("expression: unexpected trailing token %q", p.peek().text)
	}
	return node, nil
}

func (p *parser) peek() token { return p.toks[p.pos] }
func (p *parser) next() token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *parser) parseExpr(minPrec int) (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t.kind != tokOp {
			break
		}
		prec, ok := precedence[t.text]
		if !ok || prec < minPrec {
			break
		}
		op := p.next().text
		right, err := p.parseExpr(prec + 1)
		if err != nil {
			return nil, err
		}
		left = Binary{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (Node, error) {
	t := p.peek()
	if t.kind == tokOp && (t.text == "!" || t.text == "-") {
		p.next()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return Unary{Op: t.text, Expr: expr}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Node, error) {
	t := p.next()
	switch t.kind {
	case tokNumber:
		var f float64
		fmt.Sscanf(t.text, "%g", &f)
		return NumberLit{Value: f}, nil
	case tokString:
		return StringLit{Value: t.text}, nil
	case tokLParen:
		expr, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokRParen {
			return nil, fmt.Errorf("expression: expected ')'")
		}
		p.next()
		return expr, nil
	case tokIdent:
		if p.peek().kind == tokLParen {
			p.next() // consume '('
			var args []Node
			if p.peek().kind != tokRParen {
				for {
					arg, err := p.parseExpr(0)
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.peek().kind == tokComma {
						p.next()
						continue
					}
					break
				}
			}
			if p.peek().kind != tokRParen {
				return nil, fmt.Errorf("expression: expected ')' after arguments to %s", t.text)
			}
			p.next()
			return Call{Func: t.text, Args: args}, nil
		}
		name := t.text
		for p.peek().kind == tokDot {
			p.next()
			field := p.next()
			name += "." + field.text
		}
		return Ident{Name: name}, nil
	default:
		return nil, fmt.Errorf("expression: unexpected token %q", t.text)
	}
}
