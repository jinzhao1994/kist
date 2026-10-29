package structured

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func model(t *testing.T, fields string) Document {
	t.Helper()
	d, err := ParseModel(`{"fields":` + fields + `}`)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func encoded(t *testing.T, d Document) string {
	t.Helper()
	v, err := Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestStrictModelValidation(t *testing.T) {
	valid := `{"fields":[{"key":"name","label":"姓名","type":"text","value":"Example Person"}]}`
	for _, raw := range []string{
		`{"fields":[],"fields":[]}`, valid + valid,
		`{"fields":null}`, `{"fields":[],"extra":true}`,
		`{"version":1,"fields":[]}`,
		strings.Replace(valid, `"value":"Example Person"`, `"value":"Example Person","source":"user"`, 1),
		strings.Replace(valid, `"value":"Example Person"`, `"value":"Example Person","deleted":true`, 1),
		strings.Replace(valid, `"key":"name"`, `"key":"__proto__"`, 1),
		strings.Replace(valid, `"key":"name"`, `"key":"name/path"`, 1),
		strings.Replace(valid, `"label":"姓名"`, `"label":""`, 1),
		strings.Replace(valid, `"value":"Example Person"`, `"value":""`, 1),
		strings.Replace(valid, `"type":"text"`, `"type":"date"`, 1),
		strings.Replace(valid, `"value":"Example Person"`, `"value":"`+strings.Repeat("a", 4097)+`"`, 1),
		`{"fields":[{"key":"people","label":"People","type":"array","children":[{"key":"person_1","label":"Person","type":"object"}]}]}`,
		`{"fields":[{"key":"x","label":"X","type":"object","value":"bad"}]}`,
		`{"fields":[{"key":"x","label":"X","type":"text","value":"a","children":[{"key":"y","label":"Y","type":"text","value":"b"}]}]}`,
	} {
		if _, err := ParseModel(raw); err == nil {
			t.Fatal("accepted malformed model output")
		}
	}
	d, err := ParseModel(valid)
	if err != nil || d.Fields[0].Source != "ai" {
		t.Fatal(err)
	}
	if _, err := Parse(encoded(t, d)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		kind, value string
		ok          bool
	}{
		{"integer", "28", true}, {"integer", "0028", false}, {"integer", "9223372036854775808", false},
		{"number", "93.5", true}, {"number", "NaN", false}, {"number", "1e999", false},
		{"text", "00001234567890123456789", true},
		{"date", "2016", true}, {"date", "2016-07", true}, {"date", "2016-07-04", true},
		{"date", "2016-02-30", false}, {"date", "0000", false}, {"date", "2016-7", false},
		{"duration", "01:50:25", true}, {"duration", "01:61:00", false},
		{"boolean", "false", true}, {"boolean", "yes", false},
	} {
		_, err := ParseModel(fmt.Sprintf(`{"fields":[{"key":"value","label":"Value","type":%q,"value":%q}]}`, tc.kind, tc.value))
		if (err == nil) != tc.ok {
			t.Errorf("%s %s: %v", tc.kind, tc.value, err)
		}
	}
}

func TestMergePreservesHumanValuesMissingFactsAndTombstones(t *testing.T) {
	initial := model(t, `[
 {"key":"holder","label":"持有人","type":"object","children":[
 {"key":"name","label":"姓名","type":"text","value":"Incorrect"},
 {"key":"number","label":"号码","type":"text","value":"00123"}]},
 {"key":"issued_on","label":"签发日期","type":"date","value":"2020-05"},
 {"key":"issuer","label":"机构","type":"text","value":"Old"}]`)
	corrected, err := Edit(initial, "/holder/name", "Correct Person", false)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := Edit(corrected, "/holder/number", "", true)
	if err != nil {
		t.Fatal(err)
	}
	next := model(t, `[
 {"key":"holder","label":"持有人","type":"object","children":[
 {"key":"name","label":"姓名","type":"text","value":"Wrong Again"},
 {"key":"number","label":"号码","type":"text","value":"00999"}]},
 {"key":"issuer","label":"机构","type":"text","value":"New"}]`)
	before := encoded(t, deleted)
	merged, err := Merge(deleted, next)
	if err != nil {
		t.Fatal(err)
	}
	if encoded(t, deleted) != before {
		t.Fatal("merge mutated snapshot")
	}
	byKey := map[string]Field{}
	for _, f := range merged.Fields {
		byKey[f.Key] = f
	}
	if byKey["issued_on"].Value != "2020-05" || byKey["issuer"].Value != "New" {
		t.Fatal("omitted or AI field wrong")
	}
	holder := byKey["holder"]
	if holder.Children[0].Value != "Correct Person" || holder.Children[0].Source != "user" || !holder.Children[1].Deleted {
		t.Fatal("human protection lost")
	}
	suppressed, err := Edit(merged, "/holder", "", true)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Merge(suppressed, next)
	if err != nil {
		t.Fatal(err)
	}
	if encoded(t, again) != encoded(t, suppressed) {
		t.Fatal("suppressed subtree changed")
	}
	if strings.Contains(Outline(merged), "Correct Person") || strings.Contains(Outline(merged), "00123") {
		t.Fatal("outline leaked values")
	}
}

func TestArrayIdentitySurvivesReordering(t *testing.T) {
	item := func(key, name, score string) string {
		return fmt.Sprintf(`{"key":%q,"label":%q,"type":"object","children":[{"key":"name","label":"Name","type":"text","value":%q},{"key":"score","label":"Score","type":"number","value":%q}]}`, key, name, name, score)
	}
	input := func(items string) Document {
		return model(t, `[{"key":"results","label":"Results","type":"array","children":[`+items+`]}]`)
	}
	first := input(item("alice_exam", "Alice", "90") + "," + item("bob_exam", "Bob", "70"))
	edited, err := Edit(first, "/results/alice_exam/score", "95", false)
	if err != nil {
		t.Fatal(err)
	}
	reordered := input(item("bob_exam", "Bob", "72") + "," + item("alice_exam", "Alice", "91"))
	merged, err := Merge(edited, reordered)
	if err != nil {
		t.Fatal(err)
	}
	raw := encoded(t, merged)
	if !strings.Contains(raw, `"value":"95","source":"user"`) || !strings.Contains(raw, `"value":"72"`) {
		t.Fatal("array edit applied to wrong object")
	}
	if _, err := Edit(merged, "/results/0/score", "99", false); err == nil {
		t.Fatal("accepted positional edit")
	}
}

func TestStructuralChangesAndResourceBounds(t *testing.T) {
	d := model(t, `[{"key":"name","label":"Name","type":"text","value":"A"}]`)
	for _, next := range []Document{
		model(t, `[{"key":"name","label":"Name","type":"object"}]`),
		model(t, `[{"key":"holder_name","label":"Name","type":"text","value":"B"}]`),
	} {
		if _, err := Merge(d, next); err == nil {
			t.Fatal("accepted ambiguous structural change")
		}
	}
	children := `{"key":"leaf","label":"Leaf","type":"text","value":"x"}`
	for range MaxDepth {
		children = `{"key":"object","label":"Object","type":"object","children":[` + children + `]}`
	}
	if _, err := ParseModel(`{"fields":[` + children + `]}`); err == nil {
		t.Fatal("accepted excessive nesting")
	}
	fields := []string{}
	for i := range MaxNodes + 1 {
		fields = append(fields, fmt.Sprintf(`{"key":"f%d","label":"Field","type":"text","value":"x"}`, i))
	}
	if _, err := ParseModel(`{"fields":[` + strings.Join(fields, ",") + `]}`); err == nil {
		t.Fatal("accepted excessive fields")
	}
	if _, err := ParseModel(strings.Repeat(" ", MaxBytes+1)); err == nil {
		t.Fatal("accepted excessive bytes")
	}
	duplicate := `{"key":"name","label":"Name","type":"text","value":"B"}`
	if _, err := ParseModel(`{"fields":[` + duplicate + `,` + duplicate + `]}`); err == nil {
		t.Fatal("accepted duplicate semantic keys")
	}
}

func TestFieldKeyErrorsIdentifyPositionsWithoutValues(t *testing.T) {
	for _, tc := range []struct {
		name, fields, want string
	}{
		{"invalid", `[{"key":"PRIVATE-123","label":"Private label","type":"text","value":"Sensitive value"}]`, "invalid structured facts at /: invalid field key at sibling position 1"},
		{"duplicate", `[{"key":"number","label":"Primary number","type":"text","value":"PRIVATE-123"},{"key":"name","label":"Name","type":"text","value":"Sensitive value"},{"key":"number","label":"File number","type":"text","value":"PRIVATE-456"}]`, "invalid structured facts at /: repeated field key at sibling positions 1 and 3"},
		{"nested", `[{"key":"record","label":"Record","type":"object","children":[{"key":"date","label":"Start","type":"date","value":"2031-05-03"},{"key":"date","label":"End","type":"date","value":"2031-05-05"}]}]`, "invalid structured facts at /record: repeated field key at sibling positions 1 and 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseModel(`{"fields":` + tc.fields + `}`)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestDistinctFactRolesAndRecordScopes(t *testing.T) {
	d := model(t, `[
 {"key":"number","label":"Number","type":"text","value":"00123"},
 {"key":"file_number","label":"File number","type":"text","value":"00456"},
 {"key":"date","label":"Start date","type":"date","value":"2031-05-03"},
 {"key":"end_date","label":"End date","type":"date","value":"2031-05-05"},
 {"key":"related","label":"Related record","type":"object","children":[
   {"key":"number","label":"Number","type":"text","value":"00789"}]}]`)
	roundTrip, err := Parse(encoded(t, d))
	if err != nil || !reflect.DeepEqual(d, roundTrip) {
		t.Fatal("distinct identifiers, date endpoints or scoped keys were lost", err)
	}
}

func TestEncodeDoesNotMutateInput(t *testing.T) {
	d := model(t, `[{"key":"z","label":"Z","type":"text","value":"z"},{"key":"a","label":"A","type":"text","value":"a"}]`)
	original := append([]Field(nil), d.Fields...)
	raw := encoded(t, d)
	if !reflect.DeepEqual(original, d.Fields) {
		t.Fatal("encoding mutated input")
	}
	roundTrip, err := Parse(raw)
	if err != nil || !reflect.DeepEqual(original, roundTrip.Fields) {
		t.Fatal("encoding changed source reading order")
	}
}

func TestMergeUsesSourceOrderWithoutLosingOmittedOrHumanFields(t *testing.T) {
	before := model(t, `[
 {"key":"number","label":"Number","type":"text","value":"00123"},
 {"key":"name","label":"Name","type":"text","value":"Original"},
 {"key":"date","label":"Date","type":"date","value":"2025"}]`)
	before, err := Edit(before, "/name", "Correct Person", false)
	if err != nil {
		t.Fatal(err)
	}
	next := model(t, `[
 {"key":"date","label":"Date","type":"date","value":"2025-06"},
 {"key":"name","label":"Name","type":"text","value":"Wrong Person"}]`)
	merged, err := Merge(before, next)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Fields[0].Key != "date" || merged.Fields[1].Key != "name" || merged.Fields[2].Key != "number" {
		t.Fatal("source order or omitted field was lost")
	}
	if merged.Fields[1].Value != "Correct Person" || merged.Fields[1].Source != "user" || merged.Fields[2].Value != "00123" {
		t.Fatal("human or omitted value was overwritten")
	}
	if before.Fields[0].Key != "number" {
		t.Fatal("merge reordered its input snapshot")
	}
}

func TestMergeRejectsHumanChildrenInNewContainer(t *testing.T) {
	before := model(t, `[]`)
	next := model(t, `[{"key":"holder","label":"Holder","type":"object","children":[{"key":"name","label":"Name","type":"text","value":"Example"}]}]`)
	next.Fields[0].Children[0].Source = "user"
	if _, err := Merge(before, next); err == nil {
		t.Fatal("accepted forged human provenance in new subtree")
	}
}
