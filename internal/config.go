package python

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
	"github.com/sqlc-dev/plugin-sdk-go/sdk"
)

type Config struct {
	EmitExactTableNames         bool              `json:"emit_exact_table_names"`
	EmitSyncQuerier             bool              `json:"emit_sync_querier"`
	EmitAsyncQuerier            bool              `json:"emit_async_querier"`
	Package                     string            `json:"package"`
	Out                         string            `json:"out"`
	EmitPydanticModels          bool              `json:"emit_pydantic_models"`
	EmitStrEnum                 bool              `json:"emit_str_enum"`
	QueryParameterLimit         *int32            `json:"query_parameter_limit"`
	InflectionExcludeTableNames []string          `json:"inflection_exclude_table_names"`
	Overrides                   []Override        `json:"overrides"`
	Rename                      map[string]string `json:"rename"`
	OmitUnusedStructs           bool              `json:"omit_unused_structs"`
	OmitSqlcVersion             bool              `json:"omit_sqlc_version"`
	EmitModernTypes             bool              `json:"emit_modern_types"`
	EmitGenericQuerier          bool              `json:"emit_generic_querier"`
	EmitAwareDatetime           bool              `json:"emit_aware_datetime"`
	EmitQuerierProtocol         bool              `json:"emit_querier_protocol"`
	EmitQueryErrors             bool              `json:"emit_query_errors"`
}

type Override struct {
	DBType   string `json:"db_type,omitempty"`
	Column   string `json:"column,omitempty"`
	Nullable bool   `json:"nullable,omitempty"`
	PyType   string `json:"py_type,omitempty"`
	PyImport string `json:"py_import,omitempty"`

	// schema, table and column glob patterns, set when Column is used
	columnPatterns []string
}

// globalOptions is the subset of sqlc's top-level `options.<plugin>` block
// that applies to every codegen entry.
type globalOptions struct {
	Overrides []Override        `json:"overrides"`
	Rename    map[string]string `json:"rename"`
}

func parseConfig(req *plugin.GenerateRequest) (Config, error) {
	var conf Config
	if len(req.PluginOptions) > 0 {
		if err := json.Unmarshal(req.PluginOptions, &conf); err != nil {
			return conf, err
		}
	}
	if len(req.GlobalOptions) > 0 {
		var global globalOptions
		if err := json.Unmarshal(req.GlobalOptions, &global); err != nil {
			return conf, err
		}
		conf.Overrides = append(conf.Overrides, global.Overrides...)
		if len(global.Rename) > 0 {
			if conf.Rename == nil {
				conf.Rename = map[string]string{}
			}
			for k, v := range global.Rename {
				conf.Rename[k] = v
			}
		}
	}

	defaultSchema := "public"
	if req.Catalog != nil && req.Catalog.DefaultSchema != "" {
		defaultSchema = req.Catalog.DefaultSchema
	}
	for i := range conf.Overrides {
		if err := conf.Overrides[i].parse(defaultSchema); err != nil {
			return conf, fmt.Errorf("invalid override %s: %w", conf.Overrides[i].describe(), err)
		}
	}

	if conf.EmitAwareDatetime && !conf.EmitPydanticModels {
		return conf, errors.New("emit_aware_datetime requires emit_pydantic_models")
	}
	return conf, nil
}

func (o *Override) describe() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func (o *Override) parse(defaultSchema string) error {
	switch {
	case o.Column != "" && o.DBType != "":
		return errors.New("set either column or db_type, not both")
	case o.Column == "" && o.DBType == "":
		return errors.New("set one of column or db_type")
	case strings.TrimSpace(o.PyType) == "":
		return errors.New("py_type must not be empty")
	case o.PyImport != "" && strings.Contains(o.PyType, "."):
		return errors.New("py_type must be a bare name when py_import is set")
	}
	if o.Column == "" {
		if canonical, ok := pgTypeAliases[o.DBType]; ok {
			o.DBType = canonical
		}
		return nil
	}
	parts := strings.Split(o.Column, ".")
	switch len(parts) {
	case 2:
		o.columnPatterns = []string{defaultSchema, parts[0], parts[1]}
	case 3:
		o.columnPatterns = parts
	default:
		return fmt.Errorf("column %q must have the form table.column or schema.table.column", o.Column)
	}
	for _, p := range o.columnPatterns {
		if _, err := path.Match(p, ""); err != nil {
			return fmt.Errorf("column %q: %w", o.Column, err)
		}
	}
	return nil
}

// pgTypeAliases maps the SQL spellings of built-in types to the names sqlc
// reports for columns.
var pgTypeAliases = map[string]string{
	"bigint":                      "int8",
	"integer":                     "int4",
	"int":                         "int4",
	"smallint":                    "int2",
	"boolean":                     "bool",
	"real":                        "float4",
	"double precision":            "float8",
	"decimal":                     "numeric",
	"character varying":           "varchar",
	"character":                   "bpchar",
	"char":                        "bpchar",
	"timestamp without time zone": "timestamp",
	"timestamp with time zone":    "timestamptz",
	"time without time zone":      "time",
	"time with time zone":         "timetz",
}

// moduleNames are the module-level names a querier method body may read.
// A parameter with one of these names would shadow it.
func (c Config) moduleNames() map[string]struct{} {
	names := map[string]struct{}{
		"self": {}, "sqlalchemy": {}, "models": {}, "errors": {},
		"cast": {}, "Any": {}, "List": {}, "Optional": {},
		"list": {}, "int": {}, "float": {}, "bool": {}, "str": {}, "memoryview": {},
	}
	for m := range stdlibModules {
		names[m] = struct{}{}
	}
	for _, o := range c.Overrides {
		for _, m := range rootNamePattern.FindAllStringSubmatch(o.PyType, -1) {
			names[m[1]] = struct{}{}
		}
	}
	return names
}

// rootNamePattern matches each name in a type expression that is not an
// attribute, such as dict, str and uuid in "dict[str, uuid.UUID]".
var rootNamePattern = regexp.MustCompile(`(?:^|[^.\w])([A-Za-z_]\w*)`)

func globMatch(pattern, name string) bool {
	ok, _ := path.Match(pattern, name)
	return ok
}

func (o *Override) matchesColumn(col *plugin.Column, defaultSchema string) bool {
	if o.columnPatterns == nil || col.Table == nil {
		return false
	}
	schema := col.Table.Schema
	if schema == "" {
		schema = defaultSchema
	}
	name := col.Name
	if col.OriginalName != "" {
		name = col.OriginalName
	}
	return globMatch(o.columnPatterns[0], schema) &&
		globMatch(o.columnPatterns[1], col.Table.Name) &&
		globMatch(o.columnPatterns[2], name)
}

func (o *Override) matchesType(col *plugin.Column) bool {
	if o.DBType == "" {
		return false
	}
	typ := sdk.DataType(col.Type)
	if typ != o.DBType && typ != "pg_catalog."+o.DBType {
		return false
	}
	return o.Nullable == !col.NotNull
}

// findOverride returns the override for col: column entries first, then
// db_type entries, each in config order.
func (c Config) findOverride(col *plugin.Column, defaultSchema string) *Override {
	for i := range c.Overrides {
		if c.Overrides[i].matchesColumn(col, defaultSchema) {
			return &c.Overrides[i]
		}
	}
	for i := range c.Overrides {
		if c.Overrides[i].matchesType(col) {
			return &c.Overrides[i]
		}
	}
	return nil
}
