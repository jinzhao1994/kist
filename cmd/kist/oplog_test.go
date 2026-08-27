package main

import (
	"strings"
	"testing"
)

func TestLog(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "一")
	exec(t, lib, "tag", "new", "云南")

	out, err := exec(t, lib, "log")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	for _, want := range []string{"create_box", "create_tag"} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not show %q:\n%s", want, out)
		}
	}
	// Newest first, so the tag (written second) appears above the box.
	if strings.Index(out, "create_tag") > strings.Index(out, "create_box") {
		t.Errorf("log is not newest-first:\n%s", out)
	}
}

func TestLogLimit(t *testing.T) {
	lib := newLibrary(t)
	for _, n := range []string{"一", "二", "三", "四"} {
		exec(t, lib, "box", "new", n)
	}
	out, err := exec(t, lib, "log", "-n", "2")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if got := strings.Count(out, "create_box"); got != 2 {
		t.Errorf("log -n 2 printed %d ops:\n%s", got, out)
	}
}

func TestLogByBox(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "看这个")
	exec(t, lib, "box", "new", "不看这个")

	out, err := exec(t, lib, "log", "--box", "看这个")
	if err != nil {
		t.Fatalf("log --box: %v", err)
	}
	if got := strings.Count(out, "create_box"); got != 1 {
		t.Errorf("log --box showed %d ops, want just the one for that box:\n%s", got, out)
	}
}

func TestUndoFromTheCLI(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "regrettable")

	out, _ := exec(t, lib, "log", "-n", "1")
	opID := firstField(out)
	if opID == "" {
		t.Fatalf("could not read an op id out of:\n%s", out)
	}

	if _, err := exec(t, lib, "undo", opID); err != nil {
		t.Fatalf("undo %s: %v", opID, err)
	}
	list, _ := exec(t, lib, "box", "list")
	if strings.Contains(list, "regrettable") {
		t.Errorf("the box survived the undo:\n%s", list)
	}

	// spec 2.2: the history panel must be able to show what has been undone,
	// which is why reverts_id is a column.
	log, _ := exec(t, lib, "log")
	if !strings.Contains(log, "undone by") {
		t.Errorf("log does not mark the reverted op:\n%s", log)
	}
	if !strings.Contains(log, "undoes") {
		t.Errorf("log does not mark the undo op itself:\n%s", log)
	}
}

func TestUndoRefusesTwice(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "once")
	out, _ := exec(t, lib, "log", "-n", "1")
	opID := firstField(out)

	exec(t, lib, "undo", opID)
	if _, err := exec(t, lib, "undo", opID); err == nil {
		t.Error("undoing the same op twice succeeded")
	}
}

func TestUndoRejectsANonNumericID(t *testing.T) {
	lib := newLibrary(t)
	_, err := exec(t, lib, "undo", "not-a-number")
	if err == nil {
		t.Fatal("a non-numeric op id was accepted")
	}
	if !strings.Contains(err.Error(), "kist log") {
		t.Errorf("error = %q, want it to point at `kist log`", err)
	}
}

// firstField returns the first whitespace-separated token of the first
// non-empty line, which is where `kist log` puts the op id.
func firstField(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}
