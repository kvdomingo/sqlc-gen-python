package python

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/sqlc-dev/plugin-sdk-go/metadata"
	"github.com/sqlc-dev/plugin-sdk-go/plugin"
	"github.com/sqlc-dev/plugin-sdk-go/sdk"

	pyast "github.com/sqlc-dev/sqlc-gen-python/internal/ast"
	"github.com/sqlc-dev/sqlc-gen-python/internal/inflection"
	"github.com/sqlc-dev/sqlc-gen-python/internal/poet"
	pyprint "github.com/sqlc-dev/sqlc-gen-python/internal/printer"
)

type Constant struct {
	Name  string
	Type  string
	Value string
}

type Enum struct {
	Name      string
	Comment   string
	Constants []Constant
}

type pyType struct {
	// InnerType is a dotted name such as "datetime.datetime" or
	// "models.Status", or a bare name imported from Import.
	InnerType string
	Import    string
	ArrayDims int
	IsNull    bool
}

type Field struct {
	Name    string
	Type    pyType
	Comment string
	// Embed is the model built from this field's run of row columns when
	// the query used sqlc.embed().
	Embed *Struct
}

type Struct struct {
	Table   *plugin.Identifier
	Name    string
	Fields  []Field
	Comment string
}

type QueryValue struct {
	Emit   bool
	Name   string
	Struct *Struct
	Typ    pyType
}

func (v QueryValue) EmitStruct() bool {
	return v.Emit
}

func (v QueryValue) IsStruct() bool {
	return v.Struct != nil
}

func (v QueryValue) isEmpty() bool {
	return v.Typ == (pyType{}) && v.Name == "" && v.Struct == nil
}

func (v QueryValue) annotation(f *pyFile) *pyast.Node {
	if v.Typ != (pyType{}) {
		return f.annotation(v.Typ)
	}
	if v.Struct != nil {
		if v.Emit {
			return poet.Name(v.Struct.Name)
		}
		return typeRefNode("models", v.Struct.Name)
	}
	panic("no type for QueryValue: " + v.Name)
}

// A struct used to generate methods and fields on the Queries struct
type Query struct {
	Cmd          string
	Comments     []string
	MethodName   string
	ConstantName string
	SQL          string
	SourceName   string
	Ret          QueryValue
	Args         []QueryValue
}

// takesSequence reports whether the command's method takes a sequence of
// params objects instead of a single set of parameters.
func takesSequence(cmd string) bool {
	switch cmd {
	case metadata.CmdCopyFrom, metadata.CmdBatchExec, metadata.CmdBatchOne, metadata.CmdBatchMany:
		return true
	}
	return false
}

func (q Query) addArgs(f *pyFile, args *pyast.Arguments) {
	// A single struct arg does not need to be passed as a keyword argument
	if len(q.Args) == 1 && q.Args[0].IsStruct() {
		ann := q.Args[0].annotation(f)
		if takesSequence(q.Cmd) {
			ann = f.generic("Sequence", ann)
		}
		args.Args = append(args.Args, &pyast.Arg{
			Arg:        q.Args[0].Name,
			Annotation: ann,
		})
		return
	}
	for _, a := range q.Args {
		args.KwOnlyArgs = append(args.KwOnlyArgs, &pyast.Arg{
			Arg:        a.Name,
			Annotation: a.annotation(f),
		})
	}
}

// argDictNode builds the SQLAlchemy parameter dict. base, when set, replaces
// the struct argument's name as the attribute base (the loop variable of
// sequence-taking commands).
func (q Query) argDictNode(base string) *pyast.Node {
	dict := &pyast.Dict{}
	i := 1
	for _, a := range q.Args {
		if a.isEmpty() {
			continue
		}
		if a.IsStruct() {
			name := a.Name
			if base != "" {
				name = base
			}
			for _, f := range a.Struct.Fields {
				dict.Keys = append(dict.Keys, poet.Constant(fmt.Sprintf("p%v", i)))
				dict.Values = append(dict.Values, typeRefNode(name, f.Name))
				i++
			}
		} else {
			dict.Keys = append(dict.Keys, poet.Constant(fmt.Sprintf("p%v", i)))
			dict.Values = append(dict.Values, poet.Name(a.Name))
			i++
		}
	}
	if len(dict.Keys) == 0 {
		return nil
	}
	return &pyast.Node{
		Node: &pyast.Node_Dict{
			Dict: dict,
		},
	}
}

func makePyType(req *plugin.GenerateRequest, conf Config, col *plugin.Column) pyType {
	dims := int(col.ArrayDims)
	if col.IsArray && dims == 0 {
		dims = 1
	}
	typ := pyType{
		ArrayDims: dims,
		IsNull:    !col.NotNull,
	}
	if o := conf.findOverride(col, req.Catalog.DefaultSchema); o != nil {
		typ.InnerType = o.PyType
		typ.Import = o.PyImport
		return typ
	}
	typ.InnerType = pyInnerType(req, conf, col)
	return typ
}

func pyInnerType(req *plugin.GenerateRequest, conf Config, col *plugin.Column) string {
	switch req.Settings.Engine {
	case "postgresql":
		return postgresType(req, conf, col)
	default:
		log.Println("unsupported engine type")
		return "Any"
	}
}

func className(name string) string {
	return modelName(name, nil)
}

func modelName(name string, settings *plugin.Settings) string {
	out := ""
	for _, p := range strings.Split(name, "_") {
		out += strings.Title(p)
	}
	return out
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func methodName(name string) string {
	snake := matchFirstCap.ReplaceAllString(name, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

var pyIdentPattern = regexp.MustCompile("[^a-zA-Z0-9_]+")

func pyEnumValueName(value string) string {
	id := strings.Replace(value, "-", "_", -1)
	id = strings.Replace(id, ":", "_", -1)
	id = strings.Replace(id, "/", "_", -1)
	id = pyIdentPattern.ReplaceAllString(id, "")
	return strings.ToUpper(id)
}

// enumMemberNames makes every member a unique, valid identifier while the
// values keep their original strings.
func enumMemberNames(conf Config, vals []string) []string {
	names := make([]string, len(vals))
	used := map[string]bool{}
	for i, v := range vals {
		name := pyIdent(v, conf, pyEnumValueName)
		if name == "" {
			name = fmt.Sprintf("VALUE_%d", i+1)
		} else if name[0] >= '0' && name[0] <= '9' {
			name = "VALUE_" + name
		}
		names[i] = uniqueName(used, name)
	}
	return names
}

// uniqueName returns name, or name with the lowest free "_N" suffix, and
// marks the result used.
func uniqueName(used map[string]bool, name string) string {
	if used[name] {
		k := 2
		for used[fmt.Sprintf("%s_%d", name, k)] {
			k++
		}
		name = fmt.Sprintf("%s_%d", name, k)
	}
	used[name] = true
	return name
}

func buildEnums(conf Config, req *plugin.GenerateRequest) []Enum {
	var enums []Enum
	for _, schema := range req.Catalog.Schemas {
		if schema.Name == "pg_catalog" || schema.Name == "information_schema" {
			continue
		}
		for _, enum := range schema.Enums {
			var enumName string
			if schema.Name == req.Catalog.DefaultSchema {
				enumName = enum.Name
			} else {
				enumName = schema.Name + "_" + enum.Name
			}
			e := Enum{
				Name:    pyIdent(enumName, conf, className),
				Comment: enum.Comment,
			}
			for i, name := range enumMemberNames(conf, enum.Vals) {
				e.Constants = append(e.Constants, Constant{
					Name:  name,
					Value: enum.Vals[i],
					Type:  e.Name,
				})
			}
			enums = append(enums, e)
		}
	}
	if len(enums) > 0 {
		sort.Slice(enums, func(i, j int) bool { return enums[i].Name < enums[j].Name })
	}
	return enums
}

func buildModels(conf Config, req *plugin.GenerateRequest) []Struct {
	var structs []Struct
	for _, schema := range req.Catalog.Schemas {
		if schema.Name == "pg_catalog" || schema.Name == "information_schema" {
			continue
		}
		for _, table := range schema.Tables {
			var tableName string
			if schema.Name == req.Catalog.DefaultSchema {
				tableName = table.Rel.Name
			} else {
				tableName = schema.Name + "_" + table.Rel.Name
			}
			structName := tableName
			if !conf.EmitExactTableNames {
				structName = inflection.Singular(inflection.SingularParams{
					Name:       structName,
					Exclusions: conf.InflectionExcludeTableNames,
				})
			}
			s := Struct{
				Table:   &plugin.Identifier{Schema: schema.Name, Name: table.Rel.Name},
				Name:    pyIdent(structName, conf, className),
				Comment: table.Comment,
			}
			for _, column := range table.Columns {
				s.Fields = append(s.Fields, Field{
					Name:    pyIdent(column.Name, conf, nil),
					Type:    makePyType(req, conf, column),
					Comment: column.Comment,
				})
			}
			structs = append(structs, s)
		}
	}
	if len(structs) > 0 {
		sort.Slice(structs, func(i, j int) bool { return structs[i].Name < structs[j].Name })
	}
	return structs
}

func columnName(c *plugin.Column, pos int) string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("column_%d", pos+1)
}

func paramName(p *plugin.Parameter) string {
	if p.Column.Name != "" {
		return p.Column.Name
	}
	return fmt.Sprintf("dollar_%d", p.Number)
}

type pyColumn struct {
	id int32
	*plugin.Column
}

func columnsToStruct(req *plugin.GenerateRequest, conf Config, name string, columns []pyColumn, models []Struct) *Struct {
	gs := Struct{
		Name: name,
	}
	used := map[string]bool{}
	for i, c := range columns {
		field := Field{
			Name: uniqueName(used, pyIdent(columnName(c.Column, i), conf, nil)),
			Type: makePyType(req, conf, c.Column),
		}
		if c.EmbedTable != nil {
			for j := range models {
				if sdk.SameTableName(c.EmbedTable, models[j].Table, req.Catalog.DefaultSchema) {
					field.Embed = &models[j]
					field.Type = pyType{InnerType: "models." + models[j].Name}
					break
				}
			}
		}
		gs.Fields = append(gs.Fields, field)
	}
	return &gs
}

var postgresPlaceholderRegexp = regexp.MustCompile(`\B\$(\d+)\b`)

// Sqlalchemy uses ":name" for placeholders, so "$N" is converted to ":pN"
// This also means ":" has special meaning to sqlalchemy, so it must be escaped.
func sqlalchemySQL(s, engine string) string {
	s = strings.ReplaceAll(s, ":", `\:`)
	if engine == "postgresql" {
		return postgresPlaceholderRegexp.ReplaceAllString(s, ":p$1")
	}
	return s
}

func queryComments(comments []string) []string {
	out := make([]string, len(comments))
	for i, c := range comments {
		out[i] = strings.TrimPrefix(c, " ")
	}
	return out
}

func buildQueries(conf Config, req *plugin.GenerateRequest, structs []Struct) ([]Query, error) {
	qs := make([]Query, 0, len(req.Queries))
	moduleNames := conf.moduleNames()
	for _, query := range req.Queries {
		if query.Name == "" {
			continue
		}
		if query.Cmd == "" {
			continue
		}
		switch query.Cmd {
		case metadata.CmdOne, metadata.CmdMany, metadata.CmdExec, metadata.CmdExecRows, metadata.CmdExecResult,
			metadata.CmdCopyFrom, metadata.CmdBatchExec, metadata.CmdBatchOne, metadata.CmdBatchMany:
		default:
			return nil, fmt.Errorf("query %s: unsupported command %s", query.Name, query.Cmd)
		}

		methodName := methodName(query.Name)

		gq := Query{
			Cmd:          query.Cmd,
			Comments:     queryComments(query.Comments),
			MethodName:   escapeKeyword(methodName),
			ConstantName: strings.ToUpper(methodName),
			SQL:          sqlalchemySQL(query.Text, req.Settings.Engine),
			SourceName:   query.Filename,
		}

		qpl := 4
		if conf.QueryParameterLimit != nil {
			qpl = int(*conf.QueryParameterLimit)
		}
		if qpl < 0 {
			return nil, errors.New("invalid query parameter limit")
		}
		if len(query.Params) > qpl || qpl == 0 || takesSequence(query.Cmd) {
			var cols []pyColumn
			for _, p := range query.Params {
				cols = append(cols, pyColumn{
					id:     p.Number,
					Column: p.Column,
				})
			}
			gq.Args = []QueryValue{{
				Emit:   true,
				Name:   "arg",
				Struct: columnsToStruct(req, conf, query.Name+"Params", cols, structs),
			}}
		} else {
			args := make([]QueryValue, 0, len(query.Params))
			used := map[string]bool{}
			for _, p := range query.Params {
				name := pyIdent(paramName(p), conf, nil)
				if _, ok := moduleNames[name]; ok {
					name += "_"
				}
				args = append(args, QueryValue{
					Name: uniqueName(used, name),
					Typ:  makePyType(req, conf, p.Column),
				})
			}
			gq.Args = args
		}

		hasEmbed := false
		for _, c := range query.Columns {
			if c.EmbedTable != nil {
				hasEmbed = true
			}
		}

		if len(query.Columns) == 1 && !hasEmbed {
			c := query.Columns[0]
			gq.Ret = QueryValue{
				Name: columnName(c, 0),
				Typ:  makePyType(req, conf, c),
			}
		} else if len(query.Columns) > 0 {
			var gs *Struct
			var emit bool

			// A row with embeds never has a table's shape, so it is never
			// a reused model.
			for _, s := range structs {
				if hasEmbed || len(s.Fields) != len(query.Columns) {
					continue
				}
				same := true
				for i, f := range s.Fields {
					c := query.Columns[i]
					sameName := f.Name == pyIdent(columnName(c, i), conf, nil)
					sameType := f.Type == makePyType(req, conf, c)
					sameTable := sdk.SameTableName(c.Table, s.Table, req.Catalog.DefaultSchema)
					if !sameName || !sameType || !sameTable {
						same = false
					}
				}
				if same {
					s := s
					gs = &s
					break
				}
			}

			if gs == nil {
				var columns []pyColumn
				for i, c := range query.Columns {
					columns = append(columns, pyColumn{
						id:     int32(i),
						Column: c,
					})
				}
				gs = columnsToStruct(req, conf, query.Name+"Row", columns, structs)
				emit = true
			}
			gq.Ret = QueryValue{
				Emit:   emit,
				Name:   "i",
				Struct: gs,
			}
		}

		qs = append(qs, gq)
	}
	sort.Slice(qs, func(i, j int) bool { return qs[i].MethodName < qs[j].MethodName })
	return qs, nil
}

// filterUnusedStructs keeps the enums and models that a query's params,
// result or embedded tables refer to, directly or through a model field.
func filterUnusedStructs(enums []Enum, models []Struct, queries []Query) ([]Enum, []Struct) {
	keep := map[string]bool{}
	var visitStruct func(s *Struct)
	visitStruct = func(s *Struct) {
		for _, f := range s.Fields {
			keep[strings.TrimPrefix(f.Type.InnerType, "models.")] = true
			if f.Embed != nil {
				keep[f.Embed.Name] = true
				visitStruct(f.Embed)
			}
		}
	}
	visitValue := func(v QueryValue) {
		if v.Struct == nil {
			keep[strings.TrimPrefix(v.Typ.InnerType, "models.")] = true
			return
		}
		if !v.Emit {
			keep[v.Struct.Name] = true
		}
		visitStruct(v.Struct)
	}
	for _, q := range queries {
		visitValue(q.Ret)
		for _, a := range q.Args {
			visitValue(a)
		}
	}

	var keptModels []Struct
	for i := range models {
		if keep[models[i].Name] {
			keptModels = append(keptModels, models[i])
			visitStruct(&models[i])
		}
	}
	var keptEnums []Enum
	for _, e := range enums {
		if keep[e.Name] {
			keptEnums = append(keptEnums, e)
		}
	}
	return keptEnums, keptModels
}

func moduleNode(version, source string, omitVersion bool) *pyast.Module {
	mod := &pyast.Module{
		Body: []*pyast.Node{
			poet.Comment(
				"Code generated by sqlc. DO NOT EDIT.",
			),
		},
	}
	if !omitVersion {
		mod.Body = append(mod.Body,
			poet.Comment("versions:"),
			poet.Comment("  sqlc "+version),
		)
	}
	if source != "" {
		mod.Body = append(mod.Body,
			poet.Comment(
				"source: "+source,
			),
		)
	}
	return mod
}

func importNode(name string) *pyast.Node {
	return &pyast.Node{
		Node: &pyast.Node_Import{
			Import: &pyast.Import{
				Names: []*pyast.Node{
					{
						Node: &pyast.Node_Alias{
							Alias: &pyast.Alias{
								Name: name,
							},
						},
					},
				},
			},
		},
	}
}

func assignNode(target string, value *pyast.Node) *pyast.Node {
	return &pyast.Node{
		Node: &pyast.Node_Assign{
			Assign: &pyast.Assign{
				Targets: []*pyast.Node{
					poet.Name(target),
				},
				Value: value,
			},
		},
	}
}

func constantInt(value int) *pyast.Node {
	return &pyast.Node{
		Node: &pyast.Node_Constant{
			Constant: &pyast.Constant{
				Value: &pyast.Constant_Int{
					Int: int32(value),
				},
			},
		},
	}
}

func subscriptNode(value string, slice *pyast.Node) *pyast.Node {
	return &pyast.Node{
		Node: &pyast.Node_Subscript{
			Subscript: &pyast.Subscript{
				Value: &pyast.Name{Id: value},
				Slice: slice,
			},
		},
	}
}

func dataclassNode(name string) *pyast.ClassDef {
	return &pyast.ClassDef{
		Name: name,
		DecoratorList: []*pyast.Node{
			{
				Node: &pyast.Node_Call{
					Call: &pyast.Call{
						Func: poet.Attribute(poet.Name("dataclasses"), "dataclass"),
					},
				},
			},
		},
	}
}

func pydanticNode(name string) *pyast.ClassDef {
	return &pyast.ClassDef{
		Name: name,
		Bases: []*pyast.Node{
			poet.Attribute(poet.Name("pydantic"), "BaseModel"),
		},
	}
}

func docstringNode(text string) *pyast.Node {
	return poet.Expr(poet.Constant(text))
}

// structClassDef builds a model, params or row class.
func structClassDef(f *pyFile, conf Config, s *Struct) *pyast.Node {
	var def *pyast.ClassDef
	if conf.EmitPydanticModels {
		f.importModule("pydantic")
		def = pydanticNode(s.Name)
	} else {
		f.importModule("dataclasses")
		def = dataclassNode(s.Name)
	}
	if s.Comment != "" {
		def.Body = append(def.Body, docstringNode(s.Comment))
	}
	for _, field := range s.Fields {
		def.Body = append(def.Body, poet.Node(&pyast.AnnAssign{
			Target:     &pyast.Name{Id: field.Name},
			Annotation: f.annotation(field.Type),
			Comment:    field.Comment,
		}))
	}
	return poet.Node(def)
}

func typeRefNode(base string, parts ...string) *pyast.Node {
	n := poet.Name(base)
	for _, p := range parts {
		n = poet.Attribute(n, p)
	}
	return n
}

func connMethodNode(method, name string, arg *pyast.Node) *pyast.Node {
	args := []*pyast.Node{
		{
			Node: &pyast.Node_Call{
				Call: &pyast.Call{
					Func: typeRefNode("sqlalchemy", "text"),
					Args: []*pyast.Node{
						poet.Name(name),
					},
				},
			},
		},
	}
	if arg != nil {
		args = append(args, arg)
	}
	return &pyast.Node{
		Node: &pyast.Node_Call{
			Call: &pyast.Call{
				Func: typeRefNode("self", "_conn", method),
				Args: args,
			},
		},
	}
}

func buildModelsTree(ctx *pyTmplCtx) *pyast.Node {
	f := newPyFile(ctx.C, true)
	if ctx.C.EmitPydanticModels {
		f.importModule("pydantic")
	} else {
		f.importModule("dataclasses")
	}
	if len(ctx.Enums) > 0 {
		f.importModule("enum")
	}

	var body []*pyast.Node
	for _, e := range ctx.Enums {
		bases := []*pyast.Node{
			poet.Name("str"),
			poet.Attribute(poet.Name("enum"), "Enum"),
		}
		if ctx.C.EmitStrEnum {
			// override the bases to emit enum.StrEnum (only support in Python >=3.11)
			bases = []*pyast.Node{
				poet.Attribute(poet.Name("enum"), "StrEnum"),
			}
		}
		def := &pyast.ClassDef{
			Name:  e.Name,
			Bases: bases,
		}
		if e.Comment != "" {
			def.Body = append(def.Body, docstringNode(e.Comment))
		}
		for _, c := range e.Constants {
			def.Body = append(def.Body, assignNode(c.Name, poet.Constant(c.Value)))
		}
		body = append(body, poet.Node(def))
	}

	for i := range ctx.Models {
		body = append(body, structClassDef(f, ctx.C, &ctx.Models[i]))
	}

	mod := moduleNode(ctx.SqlcVersion, "", ctx.C.OmitSqlcVersion)
	mod.Body = append(mod.Body, importGroup(f.std), importGroup(f.pkg))
	mod.Body = append(mod.Body, body...)
	return poet.Node(mod)
}

// querierBuilder builds the methods of Querier (async false) or
// AsyncQuerier (async true), so both stay in step for every command.
type querierBuilder struct {
	f     *pyFile
	conf  Config
	async bool
}

type querierMethod struct {
	name     string
	args     *pyast.Arguments
	body     []*pyast.Node
	returns  *pyast.Node
	asyncGen bool
}

func (m querierMethod) node(async bool) *pyast.Node {
	if async {
		return poet.Node(&pyast.AsyncFunctionDef{
			Name:    m.name,
			Args:    m.args,
			Body:    m.body,
			Returns: m.returns,
		})
	}
	return poet.Node(&pyast.FunctionDef{
		Name:    m.name,
		Args:    m.args,
		Body:    m.body,
		Returns: m.returns,
	})
}

// protocolNode declares m with a `...` body. An async generator is declared
// with a plain def: its call type is AsyncIterator[T], whereas an async def
// stub would mean Coroutine[..., AsyncIterator[T]].
func (m querierMethod) protocolNode(async bool) *pyast.Node {
	stub := querierMethod{
		name:    m.name,
		args:    m.args,
		body:    []*pyast.Node{poet.Expr(poet.Ellipsis())},
		returns: m.returns,
	}
	return stub.node(async && !m.asyncGen)
}

func (b *querierBuilder) await(n *pyast.Node) *pyast.Node {
	if b.async {
		return poet.Await(n)
	}
	return n
}

func (b *querierBuilder) execute(q Query, params *pyast.Node) *pyast.Node {
	return b.await(connMethodNode("execute", q.ConstantName, params))
}

func (b *querierBuilder) iterator(n *pyast.Node) *pyast.Node {
	if b.async {
		return b.f.generic("AsyncIterator", n)
	}
	return b.f.generic("Iterator", n)
}

func (b *querierBuilder) forNode(target string, iter *pyast.Node, body ...*pyast.Node) *pyast.Node {
	if b.async {
		return poet.Node(&pyast.AsyncFor{Target: poet.Name(target), Iter: iter, Body: body})
	}
	return poet.Node(&pyast.For{Target: poet.Name(target), Iter: iter, Body: body})
}

// rowCell reads column i of the row, cast to its annotation so strict type
// checkers accept the untyped Row value.
func (b *querierBuilder) rowCell(t pyType, rowVar string, i int) *pyast.Node {
	return poet.Node(&pyast.Call{
		Func: b.f.typing("cast"),
		Args: []*pyast.Node{b.f.annotation(t), subscriptNode(rowVar, constantInt(i))},
	})
}

func (b *querierBuilder) structCall(s *Struct, callee *pyast.Node, rowVar string, idx *int) *pyast.Node {
	call := &pyast.Call{Func: callee}
	for _, field := range s.Fields {
		var value *pyast.Node
		if field.Embed != nil {
			value = b.structCall(field.Embed, typeRefNode("models", field.Embed.Name), rowVar, idx)
		} else {
			value = b.rowCell(field.Type, rowVar, *idx)
			*idx++
		}
		call.Keywords = append(call.Keywords, &pyast.Keyword{Arg: field.Name, Value: value})
	}
	return poet.Node(call)
}

// rowNode builds the query's return value from the row in rowVar.
func (b *querierBuilder) rowNode(v QueryValue, rowVar string) *pyast.Node {
	if !v.IsStruct() {
		return b.rowCell(v.Typ, rowVar, 0)
	}
	idx := 0
	return b.structCall(v.Struct, v.annotation(b.f), rowVar, &idx)
}

// rowcount reads result.rowcount. Session.execute is typed as returning
// Result, which lacks rowcount; the cast target is a string so it is never
// evaluated, keeping SQLAlchemy 1.4 importable.
func (b *querierBuilder) rowcount() *pyast.Node {
	b.f.typing("Any")
	return poet.Attribute(poet.Node(&pyast.Call{
		Func: b.f.typing("cast"),
		Args: []*pyast.Node{poet.Constant("sqlalchemy.engine.CursorResult[Any]"), poet.Name("result")},
	}), "rowcount")
}

func isNone(name string) *pyast.Node {
	return poet.Node(&pyast.Compare{
		Left:        poet.Name(name),
		Ops:         []*pyast.Node{poet.Is()},
		Comparators: []*pyast.Node{poet.Constant(nil)},
	})
}

// sequenceParams is the executemany parameter list for a sequence-taking
// command: one dict per item of arg.
func sequenceParams(q Query) *pyast.Node {
	dict := q.argDictNode("a")
	if dict == nil {
		dict = poet.Node(&pyast.Dict{})
	}
	return poet.ListComp(dict, poet.Name("a"), poet.Name("arg"))
}

func (b *querierBuilder) method(q Query) (querierMethod, error) {
	m := querierMethod{
		name: q.MethodName,
		args: &pyast.Arguments{Args: []*pyast.Arg{{Arg: "self"}}},
	}
	q.addArgs(b.f, m.args)

	var body []*pyast.Node
	switch q.Cmd {
	case metadata.CmdOne:
		body = append(body,
			assignNode("row", poet.Node(&pyast.Call{
				Func: poet.Attribute(b.execute(q, q.argDictNode("")), "first"),
			})),
			poet.Node(&pyast.If{
				Test: isNone("row"),
				Body: []*pyast.Node{poet.Return(poet.Constant(nil))},
			}),
			poet.Return(b.rowNode(q.Ret, "row")),
		)
		m.returns = b.f.optional(q.Ret.annotation(b.f))
	case metadata.CmdMany:
		result := b.execute(q, q.argDictNode(""))
		if b.async {
			result = poet.Await(connMethodNode("stream", q.ConstantName, q.argDictNode("")))
		}
		body = append(body,
			assignNode("result", result),
			b.forNode("row", poet.Name("result"), poet.Expr(poet.Yield(b.rowNode(q.Ret, "row")))),
		)
		m.returns = b.iterator(q.Ret.annotation(b.f))
		m.asyncGen = b.async
	case metadata.CmdExec:
		body = append(body, b.execute(q, q.argDictNode("")))
		m.returns = poet.Constant(nil)
	case metadata.CmdExecRows:
		body = append(body,
			assignNode("result", b.execute(q, q.argDictNode(""))),
			poet.Return(b.rowcount()),
		)
		m.returns = poet.Name("int")
	case metadata.CmdExecResult:
		body = append(body, poet.Return(b.execute(q, q.argDictNode(""))))
		b.f.typing("Any")
		m.returns = poet.Constant("sqlalchemy.engine.Result[Any]")
	case metadata.CmdCopyFrom:
		body = append(body,
			poet.Node(&pyast.If{
				Test: poet.Not(poet.Name("arg")),
				Body: []*pyast.Node{poet.Return(poet.Constant(0))},
			}),
			assignNode("result", b.execute(q, sequenceParams(q))),
			poet.Return(b.rowcount()),
		)
		m.returns = poet.Name("int")
	case metadata.CmdBatchExec:
		body = append(body,
			poet.Node(&pyast.If{
				Test: poet.Not(poet.Name("arg")),
				Body: []*pyast.Node{poet.Return(poet.Constant(nil))},
			}),
			b.execute(q, sequenceParams(q)),
		)
		m.returns = poet.Constant(nil)
	case metadata.CmdBatchOne:
		body = append(body, poet.Node(&pyast.For{
			Target: poet.Name("a"),
			Iter:   poet.Name("arg"),
			Body: []*pyast.Node{
				assignNode("row", poet.Node(&pyast.Call{
					Func: poet.Attribute(b.execute(q, q.argDictNode("a")), "first"),
				})),
				poet.Node(&pyast.If{
					Test:   isNone("row"),
					Body:   []*pyast.Node{poet.Expr(poet.Yield(poet.Constant(nil)))},
					OrElse: []*pyast.Node{poet.Expr(poet.Yield(b.rowNode(q.Ret, "row")))},
				}),
			},
		}))
		m.returns = b.iterator(b.f.optional(q.Ret.annotation(b.f)))
		m.asyncGen = b.async
	case metadata.CmdBatchMany:
		body = append(body, poet.Node(&pyast.For{
			Target: poet.Name("a"),
			Iter:   poet.Name("arg"),
			Body: []*pyast.Node{
				poet.Expr(poet.Yield(poet.ListComp(
					b.rowNode(q.Ret, "row"),
					poet.Name("row"),
					b.execute(q, q.argDictNode("a")),
				))),
			},
		}))
		m.returns = b.iterator(b.f.list(q.Ret.annotation(b.f)))
		m.asyncGen = b.async
	default:
		return m, fmt.Errorf("query %s: unsupported command %s", q.MethodName, q.Cmd)
	}

	// The with block encloses loops and yields, so errors raised while the
	// caller iterates a generator method are wrapped too.
	if b.conf.EmitQueryErrors {
		body = []*pyast.Node{poet.With(poet.Node(&pyast.Call{
			Func: typeRefNode("errors", "_wrap_errors"),
			Args: []*pyast.Node{poet.Constant(q.MethodName)},
		}), body...)}
	}
	if len(q.Comments) > 0 {
		m.body = append(m.body, docstringNode(strings.Join(q.Comments, "\n")))
	}
	m.body = append(m.body, body...)
	return m, nil
}

func initNode(connType *pyast.Node) *pyast.Node {
	return poet.Node(&pyast.FunctionDef{
		Name: "__init__",
		Args: &pyast.Arguments{
			Args: []*pyast.Arg{
				{Arg: "self"},
				{Arg: "conn", Annotation: connType},
			},
		},
		Body: []*pyast.Node{
			poet.Node(&pyast.Assign{
				Targets: []*pyast.Node{poet.Attribute(poet.Name("self"), "_conn")},
				Value:   poet.Name("conn"),
			}),
		},
	})
}

func (b *querierBuilder) connType() *pyast.Node {
	if b.async {
		b.f.importModule("sqlalchemy.ext.asyncio")
		return b.f.union(
			typeRefNode("sqlalchemy", "ext", "asyncio", "AsyncConnection"),
			typeRefNode("sqlalchemy", "ext", "asyncio", "AsyncSession"),
		)
	}
	b.f.importModule("sqlalchemy.orm")
	return b.f.union(
		typeRefNode("sqlalchemy", "engine", "Connection"),
		typeRefNode("sqlalchemy", "orm", "Session"),
	)
}

// classes returns the querier class, preceded by its protocol when enabled.
func (b *querierBuilder) classes(queries []Query) ([]*pyast.Node, error) {
	name := "Querier"
	if b.async {
		name = "AsyncQuerier"
	}
	cls := &pyast.ClassDef{Name: name}
	connType := b.connType()
	if b.conf.EmitGenericQuerier {
		cls.TypeParams = []*pyast.TypeVar{{Name: "T", Bound: connType}}
		connType = poet.Name("T")
	}
	cls.Body = append(cls.Body,
		poet.Node(&pyast.AnnAssign{Target: &pyast.Name{Id: "_conn"}, Annotation: connType}),
		initNode(connType),
	)
	proto := &pyast.ClassDef{Name: name + "Protocol"}
	for _, q := range queries {
		m, err := b.method(q)
		if err != nil {
			return nil, err
		}
		cls.Body = append(cls.Body, m.node(b.async))
		proto.Body = append(proto.Body, m.protocolNode(b.async))
	}
	if !b.conf.EmitQuerierProtocol {
		return []*pyast.Node{poet.Node(cls)}, nil
	}
	proto.Bases = []*pyast.Node{b.f.typing("Protocol")}
	return []*pyast.Node{poet.Node(proto), poet.Node(cls)}, nil
}

func buildQueryTree(ctx *pyTmplCtx, source string) (*pyast.Node, error) {
	f := newPyFile(ctx.C, false)
	f.importModule("sqlalchemy")

	var queries []Query
	for _, q := range ctx.Queries {
		if ctx.OutputQuery(q.SourceName) {
			queries = append(queries, q)
		}
	}

	var body []*pyast.Node
	for _, q := range queries {
		queryText := fmt.Sprintf("-- name: %s \\%s\n%s\n", q.MethodName, q.Cmd, q.SQL)
		body = append(body, assignNode(q.ConstantName, poet.Constant(queryText)))
		for _, arg := range q.Args {
			if arg.EmitStruct() {
				body = append(body, structClassDef(f, ctx.C, arg.Struct))
			}
		}
		if q.Ret.EmitStruct() {
			body = append(body, structClassDef(f, ctx.C, q.Ret.Struct))
		}
	}

	var protocols, queriers []*pyast.Node
	for _, async := range []bool{false, true} {
		if (!async && !ctx.C.EmitSyncQuerier) || (async && !ctx.C.EmitAsyncQuerier) {
			continue
		}
		b := &querierBuilder{f: f, conf: ctx.C, async: async}
		classes, err := b.classes(queries)
		if err != nil {
			return nil, err
		}
		protocols = append(protocols, classes[:len(classes)-1]...)
		queriers = append(queriers, classes[len(classes)-1])
	}
	body = append(body, protocols...)
	body = append(body, queriers...)

	local := &pyast.ImportFrom{Module: ctx.C.Package}
	if ctx.C.EmitQueryErrors {
		local.Names = append(local.Names, poet.Alias("errors"))
	}
	local.Names = append(local.Names, poet.Alias("models"))

	mod := moduleNode(ctx.SqlcVersion, source, ctx.C.OmitSqlcVersion)
	mod.Body = append(mod.Body,
		importGroup(f.std),
		importGroup(f.pkg),
		&pyast.Node{Node: &pyast.Node_ImportGroup{ImportGroup: &pyast.ImportGroup{
			Imports: []*pyast.Node{{Node: &pyast.Node_ImportFrom{ImportFrom: local}}},
		}}},
	)
	mod.Body = append(mod.Body, body...)
	return poet.Node(mod), nil
}

type pyTmplCtx struct {
	SqlcVersion string
	Models      []Struct
	Queries     []Query
	Enums       []Enum
	SourceName  string
	C           Config
}

func (t *pyTmplCtx) OutputQuery(sourceName string) bool {
	return t.SourceName == sourceName
}

func queryFileName(source string) string {
	name := source
	if !strings.HasSuffix(name, ".py") {
		name = strings.TrimSuffix(name, ".sql")
		name += ".py"
	}
	return name
}

func Generate(_ context.Context, req *plugin.GenerateRequest) (*plugin.GenerateResponse, error) {
	conf, err := parseConfig(req)
	if err != nil {
		return nil, err
	}

	enums := buildEnums(conf, req)
	models := buildModels(conf, req)
	queries, err := buildQueries(conf, req, models)
	if err != nil {
		return nil, err
	}
	if conf.OmitUnusedStructs {
		enums, models = filterUnusedStructs(enums, models, queries)
	}

	tctx := pyTmplCtx{
		Models:      models,
		Queries:     queries,
		Enums:       enums,
		SqlcVersion: req.SqlcVersion,
		C:           conf,
	}

	output := map[string]string{}
	result := pyprint.Print(buildModelsTree(&tctx), pyprint.Options{})
	tctx.SourceName = "models.py"
	output["models.py"] = string(result.Python)

	files := map[string]struct{}{}
	for _, q := range queries {
		files[q.SourceName] = struct{}{}
	}

	if conf.EmitQueryErrors {
		for source := range files {
			if queryFileName(source) == errorsFileName {
				return nil, fmt.Errorf("query file %s would overwrite %s, which emit_query_errors generates", source, errorsFileName)
			}
		}
		output[errorsFileName] = buildErrorsModule(&tctx)
	}

	for source := range files {
		tctx.SourceName = source
		tree, err := buildQueryTree(&tctx, source)
		if err != nil {
			return nil, err
		}
		result := pyprint.Print(tree, pyprint.Options{})
		output[queryFileName(source)] = string(result.Python)
	}

	resp := plugin.GenerateResponse{}

	for filename, code := range output {
		resp.Files = append(resp.Files, &plugin.File{
			Name:     filename,
			Contents: []byte(code),
		})
	}

	return &resp, nil
}
