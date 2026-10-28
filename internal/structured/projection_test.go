package structured

import (
	"testing"
	"time"
)

func TestProjectionPreservesTypesPrecisionAndEntityScopes(t *testing.T) {
	d := model(t, `[{"key":"students","label":"Students","type":"array","children":[{"key":"alice","label":"Alice","type":"object","children":[{"key":"name","label":"Name","type":"text","value":"Alice"},{"key":"results","label":"Results","type":"array","children":[{"key":"math","label":"Math","type":"object","children":[{"key":"score","label":"Score","type":"number","value":"92.5"}]}]}]}]},{"key":"document_number","label":"ID","type":"text","value":"001234"},{"key":"integer","label":"Integer","type":"integer","value":"9007199254740993"},{"key":"issued_on","label":"Issued","type":"date","value":"2024-02"},{"key":"duration","label":"Duration","type":"duration","value":"01:50:25"},{"key":"uncertain","label":"Uncertain","type":"text","value":"guess","uncertain":true}]`)
	rows, err := Project(d)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Scalar{}
	for _, r := range rows {
		got[r.Key] = r
	}
	if _, ok := got["uncertain"]; ok {
		t.Fatal("uncertainty indexed as fact")
	}
	if got["document_number"].Text != "001234" || got["document_number"].Integer != nil {
		t.Fatal("identifier coerced")
	}
	if *got["integer"].Integer != 9007199254740993 {
		t.Fatal("int64 rounded")
	}
	score := got["score"]
	if score.SchemaPath != "/students/*/results/*/score" || len(score.Scopes) != 3 || score.Scopes[1].Path != "/students/alice" || score.Scopes[2].Path != "/students/alice/results/math" {
		t.Fatal(score)
	}
	date := got["issued_on"]
	if time.Unix(*date.DateLower, 0).UTC().Format("2006-01-02") != "2024-02-01" || time.Unix(*date.DateUpper, 0).UTC().Format("2006-01-02") != "2024-03-01" {
		t.Fatal("lost partial precision")
	}
	if *got["duration"].Integer != 6625 {
		t.Fatal("elapsed time not seconds")
	}
	d, err = Edit(d, "/students/alice", "", true)
	if err != nil {
		t.Fatal(err)
	}
	rows, err = Project(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Key == "score" || r.Key == "name" {
			t.Fatal("deleted subtree indexed")
		}
	}
}
