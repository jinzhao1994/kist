package api

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestBoxSearch(t *testing.T) {
	s, l := testServer(t)
	ctx := t.Context()
	date := func(v string) domain.LocalDate {
		d, err := domain.ParseLocalDate(v)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	a, err := l.CreateBox(ctx, "Alpha", "Straße 100%_", date("2019"), domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := l.CreateBox(ctx, "beta", "report", date("2020-01"), date("2020-03"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := l.CreateBox(ctx, "ALPHA", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	d, err := l.CreateBox(ctx, "delta", "", domain.LocalDate{}, date("2019-04"))
	if err != nil {
		t.Fatal(err)
	}
	parent, err := l.CreateTag(ctx, "parent", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := l.CreateTag(ctx, "child", &parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	other, err := l.CreateTag(ctx, "other", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct {
		tag   domain.TagID
		boxes []domain.BoxID
	}{{child.ID, []domain.BoxID{a.ID, b.ID}}, {other.ID, []domain.BoxID{b.ID, c.ID}}} {
		if _, err := l.TagBoxes(ctx, pair.tag, pair.boxes); err != nil {
			t.Fatal(err)
		}
	}
	check := func() {
		t.Helper()
		for _, tc := range []struct {
			query string
			ids   []string
		}{
			{"text=" + url.QueryEscape("STRAẞE"), []string{a.ID.String()}},
			{"text=" + url.QueryEscape("100%_"), []string{a.ID.String()}},
			{"text=alpha", []string{a.ID.String(), c.ID.String()}},
			{encodedCondition("name_contains", "al") + "&" + encodedCondition("name_contains", "pha"), []string{a.ID.String(), c.ID.String()}},
			{encodedCondition("name_contains", "alpha") + "&" + encodedCondition("name_contains", "beta"), nil},
			{encodedCondition("name_equals", "alpha") + "&" + encodedCondition("name_equals", "beta"), nil},
			{encodedCondition("keyword", "report"), []string{b.ID.String()}},
			{encodedCondition("name_contains", "report"), nil},
			{encodedCondition("tag_all", parent.ID.String()) + "&" + encodedCondition("tag_all", other.ID.String()), []string{b.ID.String()}},
			{encodedCondition("from", "2026") + "&" + encodedCondition("to", "2020"), nil},
			{"keyword=report", []string{b.ID.String()}},
			{"name_contains=report", nil},
			{"name_equals=ALPHA", []string{a.ID.String(), c.ID.String()}},
			{"name_equals=alp", nil},
			{"keyword=alpha&keyword=" + url.QueryEscape("100%_"), []string{a.ID.String()}},
			{"keyword=alpha&name_contains=beta", nil},
			{"keyword=alpha&name_equals=ALPHA", []string{a.ID.String(), c.ID.String()}},
			{"keyword=" + url.QueryEscape("Straße,100"), nil},
			{"from=2019-03&to=2019-05", []string{a.ID.String(), d.ID.String()}},
			{"from=2020-02&to=2020-02", []string{b.ID.String()}},
			{"tag_all=" + parent.ID.String() + "," + other.ID.String(), []string{b.ID.String()}},
			{"tag_any=" + parent.ID.String() + "," + other.ID.String() + "&tag_not=" + other.ID.String(), []string{a.ID.String()}},
		} {
			got := decodeBody[boxSearchResult](t, s, "/boxes/search?"+tc.query)
			ids := map[string]bool{}
			for _, box := range got.Boxes {
				ids[box.ID] = true
			}
			if len(ids) != len(tc.ids) {
				t.Fatalf("%s: %+v", tc.query, got)
			}
			for _, id := range tc.ids {
				if !ids[id] {
					t.Fatalf("%s missing %s", tc.query, id)
				}
			}
		}
		for _, sort := range []string{"updated", "name", "activity"} {
			for _, asc := range []string{"true", "false"} {
				base := "/boxes/search?sort=" + sort + "&asc=" + asc
				all := decodeBody[boxSearchResult](t, s, base)
				var ids []string
				cursor := ""
				for i := 0; i < 6; i++ {
					page := decodeBody[boxSearchResult](t, s, base+"&limit=1&cursor="+url.QueryEscape(cursor))
					for _, box := range page.Boxes {
						ids = append(ids, box.ID)
					}
					cursor = page.Cursor
					if cursor == "" {
						break
					}
				}
				var expected []string
				for _, box := range all.Boxes {
					expected = append(expected, box.ID)
				}
				if len(ids) != 4 || !reflect.DeepEqual(ids, expected) {
					t.Fatalf("%s paged %v want %v", base, ids, expected)
				}
				if sort == "activity" && ids[len(ids)-1] != c.ID.String() {
					t.Fatal("unknown dates must sort last")
				}
			}
		}
	}
	check()
	if err := l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	check()
	cursor := decodeBody[boxSearchResult](t, s, "/boxes/search?sort=name&limit=1").Cursor
	for _, q := range []string{"limit=1001", "limit=-1", "sort=nope", "cursor=bad", "sort=activity&cursor=" + url.QueryEscape(cursor), "from=2020&to=2019", "from=garbage", "tag_all=missing"} {
		r := call(s, "GET", Prefix+"/boxes/search?"+q, "", "")
		if r.Code < 400 || r.Code >= 500 {
			t.Fatalf("%s: %d %s", q, r.Code, r.Body.String())
		}
	}
}
