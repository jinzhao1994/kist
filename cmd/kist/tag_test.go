package main

import (
	"strings"
	"testing"
)

func TestTagTree(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "地点")
	exec(t, lib, "tag", "new", "云南", "--parent", "地点")
	exec(t, lib, "tag", "new", "大理", "--parent", "云南")

	out, err := exec(t, lib, "tag", "tree")
	if err != nil {
		t.Fatalf("tag tree: %v", err)
	}
	for _, want := range []string{"地点", "云南", "大理"} {
		if !strings.Contains(out, want) {
			t.Errorf("tree is missing %q:\n%s", want, out)
		}
	}
	// Depth has to be visible, or a tree view shows nothing a list would not.
	if strings.Index(out, "大理") < strings.Index(out, "云南") {
		t.Errorf("children are not printed under their parent:\n%s", out)
	}
	if !strings.Contains(out, "    大理") {
		t.Errorf("nesting is not indented:\n%s", out)
	}
}

func TestTagNewRejectsADuplicate(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "云南")
	_, err := exec(t, lib, "tag", "new", "云南")
	if err == nil {
		t.Fatal("a duplicate tag name was accepted")
	}
	if !strings.Contains(err.Error(), "already") {
		t.Errorf("error = %q, want it to say the name is taken", err)
	}
}

func TestTagRenameAndMove(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "地点")
	exec(t, lib, "tag", "new", "人物")
	exec(t, lib, "tag", "new", "云南", "--parent", "地点")

	if _, err := exec(t, lib, "tag", "rename", "云南", "云南省"); err != nil {
		t.Fatalf("tag rename: %v", err)
	}
	if _, err := exec(t, lib, "tag", "move", "云南省", "--parent", "人物"); err != nil {
		t.Fatalf("tag move: %v", err)
	}

	out, _ := exec(t, lib, "tag", "tree")
	if !strings.Contains(out, "云南省") {
		t.Errorf("tree does not show the renamed tag:\n%s", out)
	}
}

func TestTagMoveToRoot(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "地点")
	exec(t, lib, "tag", "new", "云南", "--parent", "地点")

	if _, err := exec(t, lib, "tag", "move", "云南", "--parent", ""); err != nil {
		t.Fatalf("tag move to root: %v", err)
	}
	out, _ := exec(t, lib, "tag", "tree")
	// Both at depth zero now.
	if strings.Contains(out, "  云南") {
		t.Errorf("云南 is still indented under a parent:\n%s", out)
	}
}

func TestTagMoveRequiresParentFlag(t *testing.T) {
	// Silently doing nothing would be worse than an error: the user asked for
	// a move.
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "云南")
	if _, err := exec(t, lib, "tag", "move", "云南"); err == nil {
		t.Error("tag move without --parent succeeded")
	}
}

func TestTagRemove(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "临时")
	if _, err := exec(t, lib, "tag", "rm", "临时"); err != nil {
		t.Fatalf("tag rm: %v", err)
	}
	out, _ := exec(t, lib, "tag", "tree")
	if strings.Contains(out, "临时") {
		t.Errorf("the tag is still listed:\n%s", out)
	}
}

func TestTagRemoveRefusesWhenInUse(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "云南")
	exec(t, lib, "box", "new", "trip")
	exec(t, lib, "box", "tag", "trip", "云南")

	_, err := exec(t, lib, "tag", "rm", "云南")
	if err == nil {
		t.Fatal("removing a tag in use succeeded")
	}
	if !strings.Contains(err.Error(), "attached") {
		t.Errorf("error = %q, want it to explain what is still using the tag", err)
	}
}

func TestTagMoveRefusesACycle(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "地点")
	exec(t, lib, "tag", "new", "云南", "--parent", "地点")

	if _, err := exec(t, lib, "tag", "move", "地点", "--parent", "云南"); err == nil {
		t.Error("a cycle was accepted")
	}
}
