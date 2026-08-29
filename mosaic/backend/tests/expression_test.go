package tests

import (
	"testing"

	"mosaic/internal/expression"
)

func evalOK(t *testing.T, src string, env expression.Env) any {
	t.Helper()
	c, err := expression.Compile(src)
	if err != nil {
		t.Fatalf("compile(%q) error: %v", src, err)
	}
	v, err := c.Eval(env)
	if err != nil {
		t.Fatalf("eval(%q) error: %v", src, err)
	}
	return v
}

func TestArithmetic(t *testing.T) {
	v := evalOK(t, "price * quantity", expression.Env{"price": 3.5, "quantity": 2.0})
	if v.(float64) != 7 {
		t.Fatalf("expected 7, got %v", v)
	}
}

func TestComparisonAndBoolean(t *testing.T) {
	v := evalOK(t, "age > 18 && status == \"active\"", expression.Env{"age": 25.0, "status": "active"})
	if v != true {
		t.Fatalf("expected true, got %v", v)
	}
}

func TestStringFunctions(t *testing.T) {
	v := evalOK(t, `contains(name, "AI")`, expression.Env{"name": "MOSAIC AI Studio"})
	if v != true {
		t.Fatalf("expected true, got %v", v)
	}
}

func TestOperatorPrecedence(t *testing.T) {
	v := evalOK(t, "2 + 3 * 4", expression.Env{})
	if v.(float64) != 14 {
		t.Fatalf("expected 14 (precedence), got %v", v)
	}
}

func TestDivisionByZero(t *testing.T) {
	c, err := expression.Compile("a / b")
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	if _, err := c.Eval(expression.Env{"a": 1.0, "b": 0.0}); err == nil {
		t.Fatal("expected division by zero error")
	}
}

func TestUnknownFunctionErrors(t *testing.T) {
	c, err := expression.Compile("bogus(1)")
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	if _, err := c.Eval(expression.Env{}); err == nil {
		t.Fatal("expected error for unknown function")
	}
}
