package tests

import (
	"strings"
	"testing"

	"mosaic/internal/parser"
	"mosaic/internal/schema"
)

func TestCSVParseWithHeader(t *testing.T) {
	p, err := parser.Get("csv")
	if err != nil {
		t.Fatal(err)
	}
	src := "id,name,age\n1,Alice,30\n2,Bob,25\n"
	res, err := p.Parse(strings.NewReader(src), parser.Options{HasHeader: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res.Rows))
	}
	if res.Rows[0]["name"] != "Alice" {
		t.Fatalf("expected Alice, got %v", res.Rows[0]["name"])
	}
}

func TestCSVDelimiterAutoDetectTSV(t *testing.T) {
	p, err := parser.Get("csv")
	if err != nil {
		t.Fatal(err)
	}
	src := "id\tname\tage\n1\tAlice\t30\n2\tBob\t25\n"
	res, err := p.Parse(strings.NewReader(src), parser.Options{HasHeader: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Columns) != 3 {
		t.Fatalf("expected delimiter auto-detection to find 3 columns, got %d (%v)", len(res.Columns), res.Columns)
	}
}

func TestNDJSONStream(t *testing.T) {
	p, err := parser.Get("ndjson")
	if err != nil {
		t.Fatal(err)
	}
	src := `{"id":1,"name":"Alice"}
{"id":2,"name":"Bob"}
`
	var got []schema.Row
	err = p.Stream(strings.NewReader(src), parser.Options{}, func(r schema.Row) error {
		got = append(got, r)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
}

func TestProfilerInfersTypes(t *testing.T) {
	rows := []schema.Row{
		{"age": "30", "active": "true"},
		{"age": "25", "active": "false"},
		{"age": "", "active": "true"},
	}
	report := schema.Profile([]string{"age", "active"}, rows, 10)
	ageCol, ok := report.Schema.ColumnByName("age")
	if !ok {
		t.Fatal("expected age column")
	}
	if ageCol.Type != schema.TypeInteger {
		t.Fatalf("expected age inferred as integer, got %s", ageCol.Type)
	}
	if ageCol.Stats.NullCount != 1 {
		t.Fatalf("expected 1 null, got %d", ageCol.Stats.NullCount)
	}
}
