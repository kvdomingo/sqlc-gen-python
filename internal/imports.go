package python

import (
	"sort"
	"strings"

	pyast "github.com/sqlc-dev/sqlc-gen-python/internal/ast"
	"github.com/sqlc-dev/sqlc-gen-python/internal/poet"
)

type importSpec struct {
	Module string
	Name   string
}

// pyFile records the imports a generated module needs while its body is
// built, so imports always match what the module references.
type pyFile struct {
	modern   bool
	inModels bool
	std      map[string]importSpec
	pkg      map[string]importSpec
}

func newPyFile(conf Config, inModels bool) *pyFile {
	return &pyFile{
		modern:   conf.EmitModernTypes,
		inModels: inModels,
		std:      map[string]importSpec{},
		pkg:      map[string]importSpec{},
	}
}

var stdlibModules = map[string]struct{}{
	"abc": {}, "collections": {}, "contextlib": {}, "dataclasses": {},
	"datetime": {}, "decimal": {}, "enum": {}, "fractions": {},
	"ipaddress": {}, "json": {}, "pathlib": {}, "typing": {}, "uuid": {},
	// pydantic is third-party, but the first import group has always held it.
	"pydantic": {},
}

func (f *pyFile) group(module string) map[string]importSpec {
	top, _, _ := strings.Cut(module, ".")
	if _, ok := stdlibModules[top]; ok {
		return f.std
	}
	return f.pkg
}

func (f *pyFile) importModule(module string) {
	f.group(module)[module] = importSpec{Module: module}
}

func (f *pyFile) importName(module, name string) {
	f.group(module)[module+"."+name] = importSpec{Module: module, Name: name}
}

func (f *pyFile) typing(name string) *pyast.Node {
	f.importName("typing", name)
	return poet.Name(name)
}

// abc returns a collections.abc name, spelled from typing unless modern
// syntax is on.
func (f *pyFile) abc(name string) *pyast.Node {
	if f.modern {
		f.importName("collections.abc", name)
		return poet.Name(name)
	}
	return f.typing(name)
}

func (f *pyFile) optional(n *pyast.Node) *pyast.Node {
	if f.modern {
		return poet.BitOr(n, poet.Constant(nil))
	}
	f.typing("Optional")
	return poet.Subscript("Optional", n)
}

func (f *pyFile) list(n *pyast.Node) *pyast.Node {
	if f.modern {
		return poet.Subscript("list", n)
	}
	f.typing("List")
	return poet.Subscript("List", n)
}

func (f *pyFile) union(ns ...*pyast.Node) *pyast.Node {
	if f.modern {
		out := ns[0]
		for _, n := range ns[1:] {
			out = poet.BitOr(out, n)
		}
		return out
	}
	f.typing("Union")
	return poet.Subscript("Union", ns...)
}

// generic wraps n in a collections.abc generic: Iterator, AsyncIterator or
// Sequence.
func (f *pyFile) generic(name string, n *pyast.Node) *pyast.Node {
	f.abc(name)
	return poet.Subscript(name, n)
}

func (f *pyFile) typeName(t pyType) *pyast.Node {
	switch {
	case t.Import != "":
		f.importName(t.Import, t.InnerType)
		return poet.Name(t.InnerType)
	case t.InnerType == "Any":
		return f.typing("Any")
	case strings.HasPrefix(t.InnerType, "models."):
		if f.inModels {
			return poet.Name(strings.TrimPrefix(t.InnerType, "models."))
		}
		return poet.Name(t.InnerType)
	}
	if i := strings.LastIndex(t.InnerType, "."); i > 0 {
		f.importModule(t.InnerType[:i])
	}
	return poet.Name(t.InnerType)
}

func (f *pyFile) annotation(t pyType) *pyast.Node {
	ann := f.typeName(t)
	for i := 0; i < t.ArrayDims; i++ {
		ann = f.list(ann)
	}
	if t.IsNull {
		ann = f.optional(ann)
	}
	return ann
}

// importGroup prints straight imports before from-imports, each sorted by
// module, as isort does.
func importGroup(specs map[string]importSpec) *pyast.Node {
	var modules []string
	names := map[string][]string{}
	for _, spec := range specs {
		if spec.Name == "" {
			modules = append(modules, spec.Module)
		} else {
			names[spec.Module] = append(names[spec.Module], spec.Name)
		}
	}
	sort.Strings(modules)
	fromModules := make([]string, 0, len(names))
	for m := range names {
		fromModules = append(fromModules, m)
	}
	sort.Strings(fromModules)

	var body []*pyast.Node
	for _, m := range modules {
		body = append(body, importNode(m))
	}
	for _, m := range fromModules {
		sort.Strings(names[m])
		imp := &pyast.ImportFrom{Module: m}
		for _, name := range names[m] {
			imp.Names = append(imp.Names, poet.Alias(name))
		}
		body = append(body, &pyast.Node{Node: &pyast.Node_ImportFrom{ImportFrom: imp}})
	}
	return &pyast.Node{
		Node: &pyast.Node_ImportGroup{
			ImportGroup: &pyast.ImportGroup{Imports: body},
		},
	}
}
