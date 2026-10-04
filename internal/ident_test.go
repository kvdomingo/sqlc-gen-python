package python

import "testing"

func TestPyIdent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rename map[string]string
		want   string
	}{
		{name: "from", want: "from_"},
		{name: "class", want: "class_"},
		{name: "match", want: "match"},
		{name: "type", want: "type"},
		{name: "author_id", want: "author_id"},
		{name: "spotify_url", rename: map[string]string{"spotify_url": "spotify_link"}, want: "spotify_link"},
		{name: "source", rename: map[string]string{"source": "from"}, want: "from_"},
	} {
		got := pyIdent(tc.name, Config{Rename: tc.rename}, nil)
		if got != tc.want {
			t.Errorf("pyIdent(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestPyIdentTransform(t *testing.T) {
	conf := Config{Rename: map[string]string{"person": "Human"}}
	if got := pyIdent("person", conf, func(s string) string { return modelName(s, nil) }); got != "Human" {
		t.Errorf("rename should bypass transform, got %q", got)
	}
	if got := pyIdent("book", conf, func(s string) string { return modelName(s, nil) }); got != "Book" {
		t.Errorf("got %q", got)
	}
}

func TestEnumMemberNames(t *testing.T) {
	for _, tc := range []struct {
		vals []string
		want []string
	}{
		{vals: []string{"=", "<>"}, want: []string{"VALUE_1", "VALUE_2"}},
		{vals: []string{"in-progress", "in_progress", "in progress"}, want: []string{"IN_PROGRESS", "IN_PROGRESS_2", "INPROGRESS"}},
		{vals: []string{"a-b", "a_b", "a_b_2"}, want: []string{"A_B", "A_B_2", "A_B_2_2"}},
		{vals: []string{"1st"}, want: []string{"VALUE_1ST"}},
		{vals: []string{`say "hi"`}, want: []string{"SAYHI"}},
		{vals: []string{"in-progress", "="}, want: []string{"IN_PROGRESS", "VALUE_2"}},
	} {
		got := enumMemberNames(Config{}, tc.vals)
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("enumMemberNames(%q) = %q, want %q", tc.vals, got, tc.want)
				break
			}
		}
	}
}
