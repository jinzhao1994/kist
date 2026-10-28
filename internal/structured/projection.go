package structured

import (
	"strconv"
	"strings"
	"time"
)

// Scalar is a disposable search projection. Path identifies the exact stored
// field; SchemaPath replaces array item identities with *, for stable queries.
// Separate integer/real columns avoid rounding identifiers or int64 values.
type Scalar struct {
	Path, SchemaPath, Key, Type, Text, Unit string
	Integer                                 *int64
	Real                                    *float64
	DateLower, DateUpper                    *int64
	Scopes                                  []Scope
}

// Scope identifies an enclosing repeated entity. Joining on file + scope path
// and selecting its schema path prevents predicates matching different people.
type Scope struct{ Path, SchemaPath string }

// Project is deterministic and independent of SQLite. It emits no rows for
// deleted subtrees, uncertain values or empty values. Date bounds are UTC Unix
// seconds, [lower, upper), so partial-date precision remains queryable.
func Project(d Document) ([]Scalar, error) {
	raw, err := Encode(d)
	if err != nil {
		return nil, err
	}
	d, _ = Parse(raw)
	rows := []Scalar{}
	var walk func([]Field, string, string, bool, []Scope)
	walk = func(fields []Field, path, schema string, array bool, scopes []Scope) {
		for _, f := range fields {
			if f.Deleted {
				continue
			}
			p, s := path+"/"+f.Key, schema+"/"+f.Key
			own := append([]Scope(nil), scopes...)
			if array {
				s = schema + "/*"
				own = append(own, Scope{p, s})
			}
			if f.Type == "object" || f.Type == "array" {
				walk(f.Children, p, s, f.Type == "array", own)
				continue
			}
			if f.Uncertain || f.Value == "" {
				continue
			}
			r := Scalar{Path: p, SchemaPath: s, Key: f.Key, Type: f.Type, Text: f.Value, Unit: f.Unit, Scopes: own}
			switch f.Type {
			case "integer":
				n, _ := strconv.ParseInt(f.Value, 10, 64)
				r.Integer = &n
			case "number":
				n, _ := strconv.ParseFloat(f.Value, 64)
				r.Real = &n
			case "duration":
				parts := strings.Split(f.Value, ":")
				h, _ := strconv.ParseInt(parts[0], 10, 64)
				m, _ := strconv.ParseInt(parts[1], 10, 64)
				s, _ := strconv.ParseInt(parts[2], 10, 64)
				n := h*3600 + m*60 + s
				r.Integer = &n
			case "date":
				layout := map[int]string{4: "2006", 7: "2006-01", 10: "2006-01-02"}[len(f.Value)]
				date, _ := time.Parse(layout, f.Value)
				end := date.AddDate(0, 0, 1)
				if len(f.Value) == 4 {
					end = date.AddDate(1, 0, 0)
				} else if len(f.Value) == 7 {
					end = date.AddDate(0, 1, 0)
				}
				lo, hi := date.Unix(), end.Unix()
				r.DateLower, r.DateUpper = &lo, &hi
			}
			rows = append(rows, r)
		}
	}
	walk(d.Fields, "", "", false, []Scope{{"/", "/"}})
	return rows, nil
}
