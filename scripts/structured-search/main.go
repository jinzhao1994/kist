// Command structured-search measures a disposable SQLite projection using only
// synthetic documents. It never opens a kist library or writes production schema.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jinzhao1994/kist/internal/structured"
	_ "modernc.org/sqlite"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func field(key, typ, value string) structured.Field {
	return structured.Field{Key: key, Label: key, Type: typ, Value: value, Source: "ai"}
}
func sample(i int) structured.Document {
	person := func(key, name, score string) structured.Field {
		return structured.Field{Key: key, Label: key, Type: "object", Source: "ai", Children: []structured.Field{field("name", "text", name), field("score", "number", score)}}
	}
	return structured.Document{Version: 1, Fields: []structured.Field{
		field("document_number", "text", fmt.Sprintf("%012d", i)), field("issued_on", "date", fmt.Sprintf("2024-%02d", i%12+1)), field("rank", "integer", fmt.Sprint(i%100+1)),
		{Key: "results", Label: "Results", Type: "array", Source: "ai", Children: []structured.Field{person("candidate_a", "Candidate A", fmt.Sprint(i%100)), person("candidate_b", "Candidate B", "95")}},
	}}
}
func main() {
	const n = 10000
	dir, err := os.MkdirTemp("", "kist-synthetic-search-")
	must(err)
	defer os.RemoveAll(dir)
	db, err := sql.Open("sqlite", filepath.Join(dir, "experiment.db"))
	must(err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=OFF;
 CREATE TABLE documents(file_id INTEGER PRIMARY KEY,payload TEXT NOT NULL);
 CREATE TABLE facts(file_id INTEGER NOT NULL,path TEXT NOT NULL,schema_path TEXT NOT NULL,key TEXT NOT NULL,kind TEXT NOT NULL,text_value TEXT NOT NULL,int_value INTEGER,real_value REAL,date_lower INTEGER,date_upper INTEGER,PRIMARY KEY(file_id,path)) WITHOUT ROWID;
 CREATE TABLE scopes(file_id INTEGER NOT NULL,path TEXT NOT NULL,scope_path TEXT NOT NULL,scope_schema TEXT NOT NULL,PRIMARY KEY(file_id,path,scope_path)) WITHOUT ROWID;
 CREATE INDEX exact_value ON facts(key,kind,text_value,file_id,path);
 CREATE INDEX integer_value ON facts(key,int_value,file_id,path) WHERE int_value IS NOT NULL;
 CREATE INDEX real_value ON facts(key,real_value,file_id,path) WHERE real_value IS NOT NULL;
 CREATE INDEX date_range ON facts(key,date_lower,date_upper,file_id,path) WHERE date_lower IS NOT NULL;
 CREATE INDEX scope_identity ON scopes(scope_schema,scope_path,file_id,path);`)
	must(err)
	started := time.Now()
	tx, err := db.Begin()
	must(err)
	doc, err := tx.Prepare("INSERT INTO documents VALUES (?,?)")
	must(err)
	fact, err := tx.Prepare("INSERT INTO facts VALUES (?,?,?,?,?,?,?,?,?,?)")
	must(err)
	scope, err := tx.Prepare("INSERT INTO scopes VALUES (?,?,?,?)")
	must(err)
	rows := 0
	scopeRows := 0
	for i := 0; i < n; i++ {
		d := sample(i)
		raw, err := structured.Encode(d)
		must(err)
		_, err = doc.Exec(i, raw)
		must(err)
		projected, err := structured.Project(d)
		must(err)
		for _, r := range projected {
			_, err = fact.Exec(i, r.Path, r.SchemaPath, r.Key, r.Type, r.Text, r.Integer, r.Real, r.DateLower, r.DateUpper)
			must(err)
			rows++
			for _, s := range r.Scopes {
				_, err = scope.Exec(i, r.Path, s.Path, s.SchemaPath)
				must(err)
				scopeRows++
			}
		}
	}
	must(doc.Close())
	must(fact.Close())
	must(scope.Close())
	must(tx.Commit())
	load := time.Since(started)
	_, err = db.Exec("ANALYZE")
	must(err)
	type result struct {
		Name     string   `json:"name"`
		Count    int      `json:"count"`
		MedianMS float64  `json:"median_ms"`
		Plan     []string `json:"plan"`
	}
	measure := func(name, q string, args ...any) result {
		r := result{Name: name}
		times := []float64{}
		for range 7 {
			start := time.Now()
			must(db.QueryRow(q, args...).Scan(&r.Count))
			times = append(times, float64(time.Since(start).Microseconds())/1000)
		}
		sort.Float64s(times)
		r.MedianMS = times[len(times)/2]
		plan, err := db.Query("EXPLAIN QUERY PLAN "+q, args...)
		must(err)
		for plan.Next() {
			var id, parent, unused int
			var detail string
			must(plan.Scan(&id, &parent, &unused, &detail))
			r.Plan = append(r.Plan, detail)
		}
		must(plan.Close())
		return r
	}
	results := []result{
		measure("identifier-json-scan", `SELECT count(DISTINCT d.file_id) FROM documents d, json_tree(d.payload) j WHERE j.key='key' AND j.value='document_number' AND json_extract(d.payload,j.path||'.value')=?`, "000000004242"),
		measure("identifier-index", `SELECT count(*) FROM facts WHERE key='document_number' AND kind='text' AND text_value=?`, "000000004242"),
		measure("name-index", `SELECT count(*) FROM facts WHERE key='name' AND kind='text' AND text_value=?`, "Candidate A"),
		measure("rank-integer-range", `SELECT count(*) FROM facts WHERE key='rank' AND int_value<=?`, 10),
		measure("score-real-range", `SELECT count(*) FROM facts WHERE key='score' AND real_value>=?`, 90),
		measure("date-overlap", `SELECT count(*) FROM facts WHERE key='issued_on' AND date_lower<? AND date_upper>?`, time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC).Unix()),
		measure("same-entity", `SELECT count(DISTINCT a.file_id) FROM facts a JOIN scopes sa ON sa.file_id=a.file_id AND sa.path=a.path AND sa.scope_schema='/results/*' JOIN scopes sb ON sb.file_id=sa.file_id AND sb.scope_path=sa.scope_path AND sb.scope_schema=sa.scope_schema JOIN facts b ON b.file_id=sb.file_id AND b.path=sb.path WHERE a.key='name' AND a.kind='text' AND a.text_value='Candidate A' AND b.key='score' AND b.real_value>=90`),
		measure("incorrect-file-only-join", `SELECT count(DISTINCT a.file_id) FROM facts a JOIN facts b ON b.file_id=a.file_id WHERE a.key='name' AND a.kind='text' AND a.text_value='Candidate A' AND b.key='score' AND b.real_value>=90`),
	}
	if results[0].Count != 1 || results[1].Count != 1 || results[6].Count != 1000 || results[7].Count != 10000 {
		panic("projection correctness assertion failed")
	}
	var pages, pageSize int64
	must(db.QueryRow("PRAGMA page_count").Scan(&pages))
	must(db.QueryRow("PRAGMA page_size").Scan(&pageSize))
	var version string
	must(db.QueryRow("SELECT sqlite_version()").Scan(&version))
	out := map[string]any{"documents": n, "scalar_rows": rows, "scope_rows": scopeRows, "sqlite_version": version, "build_ms": float64(load.Milliseconds()), "database_bytes": pages * pageSize, "queries": results}
	encoded, err := json.MarshalIndent(out, "", "  ")
	must(err)
	fmt.Println(string(encoded))
}
