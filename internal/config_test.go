package python

import (
	"strings"
	"testing"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

func TestParseConfigGlobalMerge(t *testing.T) {
	req := &plugin.GenerateRequest{
		Catalog: &plugin.Catalog{DefaultSchema: "public"},
		PluginOptions: []byte(`{
			"overrides": [{"db_type": "uuid", "py_type": "local.Id"}],
			"rename": {"a": "local_a", "b": "local_b"}
		}`),
		GlobalOptions: []byte(`{
			"overrides": [{"db_type": "uuid", "py_type": "global.Id"}, {"db_type": "citext", "py_type": "my_lib.CIText"}],
			"rename": {"b": "global_b", "c": "global_c"}
		}`),
	}
	conf, err := parseConfig(req)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range conf.Overrides {
		got = append(got, o.PyType)
	}
	if strings.Join(got, ",") != "local.Id,global.Id,my_lib.CIText" {
		t.Errorf("override order = %v", got)
	}
	col := &plugin.Column{Type: &plugin.Identifier{Name: "uuid"}, NotNull: true}
	if o := conf.findOverride(col, "public"); o == nil || o.PyType != "local.Id" {
		t.Errorf("local override should win, got %+v", o)
	}
	want := map[string]string{"a": "local_a", "b": "global_b", "c": "global_c"}
	for k, v := range want {
		if conf.Rename[k] != v {
			t.Errorf("rename[%q] = %q, want %q", k, conf.Rename[k], v)
		}
	}
}

func TestParseConfigInvalidOverrides(t *testing.T) {
	for _, tc := range []struct {
		override string
		err      string
	}{
		{`{"py_type": "str"}`, "set one of column or db_type"},
		{`{"db_type": "uuid", "column": "a.b", "py_type": "str"}`, "not both"},
		{`{"db_type": "uuid", "py_type": " "}`, "py_type must not be empty"},
		{`{"db_type": "uuid", "py_type": "uuid.UUID", "py_import": "uuid"}`, "bare name"},
		{`{"column": "id", "py_type": "int"}`, "table.column"},
		{`{"column": "a.b.c.d", "py_type": "int"}`, "table.column"},
		{`{"column": "a.[b", "py_type": "int"}`, "syntax error"},
	} {
		req := &plugin.GenerateRequest{
			Catalog:       &plugin.Catalog{DefaultSchema: "public"},
			PluginOptions: []byte(`{"overrides": [` + tc.override + `]}`),
		}
		_, err := parseConfig(req)
		if err == nil || !strings.Contains(err.Error(), tc.err) || !strings.Contains(err.Error(), "invalid override {") {
			t.Errorf("override %s: err = %v, want it to name the entry and contain %q", tc.override, err, tc.err)
		}
	}
}

func TestParseConfigAwareDatetimeRequiresPydantic(t *testing.T) {
	req := &plugin.GenerateRequest{PluginOptions: []byte(`{"emit_aware_datetime": true}`)}
	if _, err := parseConfig(req); err == nil || !strings.Contains(err.Error(), "emit_pydantic_models") {
		t.Errorf("err = %v", err)
	}
	req.PluginOptions = []byte(`{"emit_aware_datetime": true, "emit_pydantic_models": true}`)
	if _, err := parseConfig(req); err != nil {
		t.Error(err)
	}
}

func TestFindOverride(t *testing.T) {
	conf := Config{Overrides: []Override{
		{DBType: "uuid", PyType: "my_ids.Id"},
		{Column: "users.id", PyType: "my_ids.UserId"},
		{DBType: "jsonb", PyType: "my_lib.Payload", Nullable: true},
		{Column: "*.created_*", PyType: "my_lib.Stamp"},
	}}
	for i := range conf.Overrides {
		if err := conf.Overrides[i].parse("public"); err != nil {
			t.Fatal(err)
		}
	}
	users := &plugin.Identifier{Name: "users"}
	for _, tc := range []struct {
		name string
		col  *plugin.Column
		want string
	}{
		{"column wins", &plugin.Column{Name: "id", Table: users, Type: &plugin.Identifier{Name: "uuid"}, NotNull: true}, "my_ids.UserId"},
		{"db_type", &plugin.Column{Name: "org_id", Table: users, Type: &plugin.Identifier{Name: "uuid"}, NotNull: true}, "my_ids.Id"},
		{"pg_catalog alias", &plugin.Column{Name: "x", Type: &plugin.Identifier{Schema: "pg_catalog", Name: "uuid"}, NotNull: true}, "my_ids.Id"},
		{"nullable skipped by default", &plugin.Column{Name: "x", Type: &plugin.Identifier{Name: "uuid"}}, ""},
		{"nullable entry", &plugin.Column{Name: "x", Type: &plugin.Identifier{Name: "jsonb"}}, "my_lib.Payload"},
		{"nullable entry skips not null", &plugin.Column{Name: "x", Type: &plugin.Identifier{Name: "jsonb"}, NotNull: true}, ""},
		{"glob", &plugin.Column{Name: "created_at", Table: &plugin.Identifier{Name: "posts"}, Type: &plugin.Identifier{Name: "timestamptz"}}, "my_lib.Stamp"},
		{"other schema", &plugin.Column{Name: "id", Table: &plugin.Identifier{Schema: "audit", Name: "users"}, Type: &plugin.Identifier{Name: "int8"}, NotNull: true}, ""},
	} {
		got := ""
		if o := conf.findOverride(tc.col, "public"); o != nil {
			got = o.PyType
		}
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
