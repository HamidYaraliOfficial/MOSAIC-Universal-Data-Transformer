// Package schema defines MOSAIC's universal type system and dataset schema
// representation. Every parser, transform node and export writer in the
// engine speaks this vocabulary, which is what lets a CSV file, a JSON blob
// and a SQL table all flow through the same Pipeline Canvas.
package schema

import "fmt"

// DataType is the universal MOSAIC type system. It intentionally stays small
// and closed so that every transform node can exhaustively switch over it.
type DataType string

const (
	TypeString   DataType = "string"
	TypeInteger  DataType = "integer"
	TypeFloat    DataType = "float"
	TypeBoolean  DataType = "boolean"
	TypeDate     DataType = "date"
	TypeDateTime DataType = "datetime"
	TypeTime     DataType = "time"
	TypeUUID     DataType = "uuid"
	TypeJSON     DataType = "json"
	TypeArray    DataType = "array"
	TypeObject   DataType = "object"
	TypeBinary   DataType = "binary"
	TypeNull     DataType = "null"
)

// Column describes a single field of a dataset, including everything the
// Schema Designer needs to render and edit it.
type Column struct {
	Name        string         `json:"name"`
	Type        DataType       `json:"type"`
	Nullable    bool           `json:"nullable"`
	Unique      bool           `json:"unique,omitempty"`
	Description string         `json:"description,omitempty"`
	Constraints []Constraint   `json:"constraints,omitempty"`
	Stats       *ColumnProfile `json:"stats,omitempty"`
}

// Constraint is a lightweight, user-editable validation rule attached to a
// column (e.g. min/max, regex pattern, enum of allowed values).
type Constraint struct {
	Kind  string `json:"kind"` // "min", "max", "regex", "enum", "notNull"
	Value string `json:"value"`
}

// Schema is an ordered set of columns describing a dataset shape.
type Schema struct {
	Columns []Column `json:"columns"`
}

// ColumnByName does a linear lookup; schemas are small enough (tens to a
// few hundred columns) that this beats maintaining an index map everywhere.
func (s *Schema) ColumnByName(name string) (*Column, bool) {
	for i := range s.Columns {
		if s.Columns[i].Name == name {
			return &s.Columns[i], true
		}
	}
	return nil, false
}

func (s *Schema) String() string {
	return fmt.Sprintf("Schema(%d columns)", len(s.Columns))
}

// Row is a single record flowing through the pipeline. Using a map keeps the
// engine format-agnostic (CSV rows, JSON objects and SQL rows all normalize
// into this shape) at the cost of some allocation overhead, which the
// streaming engine amortizes via row pooling (see runtime.RowPool).
type Row map[string]any

// Clone returns a shallow copy of the row, sufficient for the value-level
// mutations transform nodes perform (nodes never mutate nested structures
// in place).
func (r Row) Clone() Row {
	out := make(Row, len(r))
	for k, v := range r {
		out[k] = v
	}
	return out
}
