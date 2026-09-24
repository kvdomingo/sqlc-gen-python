package printer

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sqlc-dev/sqlc-gen-python/internal/ast"
	"github.com/sqlc-dev/sqlc-gen-python/internal/poet"
)

type testcase struct {
	Node     *ast.Node
	Expected string
}

func TestPrinter(t *testing.T) {
	for name, tc := range map[string]testcase{
		"assign": {
			Node: &ast.Node{
				Node: &ast.Node_Assign{
					Assign: &ast.Assign{
						Targets: []*ast.Node{
							{
								Node: &ast.Node_Name{
									Name: &ast.Name{Id: "FICTION"},
								},
							},
						},
						Value: &ast.Node{
							Node: &ast.Node_Constant{
								Constant: &ast.Constant{
									Value: &ast.Constant_Str{
										Str: "FICTION",
									},
								},
							},
						},
					},
				},
			},
			Expected: `FICTION = "FICTION"`,
		},
		"class-base": {
			Node: &ast.Node{
				Node: &ast.Node_ClassDef{
					ClassDef: &ast.ClassDef{
						Name: "Foo",
						Bases: []*ast.Node{
							{
								Node: &ast.Node_Name{
									Name: &ast.Name{Id: "str"},
								},
							},
							{
								Node: &ast.Node_Attribute{
									Attribute: &ast.Attribute{
										Value: &ast.Node{
											Node: &ast.Node_Name{
												Name: &ast.Name{Id: "enum"},
											},
										},
										Attr: "Enum",
									},
								},
							},
						},
					},
				},
			},
			Expected: `
class Foo(str, enum.Enum):
    pass
`,
		},
		"dataclass": {
			Node: &ast.Node{
				Node: &ast.Node_ClassDef{
					ClassDef: &ast.ClassDef{
						Name: "Foo",
						DecoratorList: []*ast.Node{
							{
								Node: &ast.Node_Name{
									Name: &ast.Name{
										Id: "dataclass",
									},
								},
							},
						},
						Body: []*ast.Node{
							{
								Node: &ast.Node_AnnAssign{
									AnnAssign: &ast.AnnAssign{
										Target: &ast.Name{Id: "bar"},
										Annotation: &ast.Node{
											Node: &ast.Node_Name{
												Name: &ast.Name{Id: "int"},
											},
										},
									},
								},
							},
							{
								Node: &ast.Node_AnnAssign{
									AnnAssign: &ast.AnnAssign{
										Target: &ast.Name{Id: "bat"},
										Annotation: &ast.Node{
											Node: &ast.Node_Subscript{
												Subscript: &ast.Subscript{
													Value: &ast.Name{Id: "Optional"},
													Slice: &ast.Node{
														Node: &ast.Node_Name{
															Name: &ast.Name{Id: "int"},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			Expected: `
@dataclass
class Foo:
    bar: int
    bat: Optional[int]
`,
		},
		"call": {
			Node: &ast.Node{
				Node: &ast.Node_Call{
					Call: &ast.Call{
						Func: &ast.Node{
							Node: &ast.Node_Alias{
								Alias: &ast.Alias{
									Name: "foo",
								},
							},
						},
					},
				},
			},
			Expected: `foo()`,
		},

		"import": {
			Node: &ast.Node{
				Node: &ast.Node_Import{
					Import: &ast.Import{
						Names: []*ast.Node{
							{
								Node: &ast.Node_Alias{
									Alias: &ast.Alias{
										Name: "foo",
									},
								},
							},
						},
					},
				},
			},
			Expected: `import foo`,
		},
		"import-from": {
			Node: &ast.Node{
				Node: &ast.Node_ImportFrom{
					ImportFrom: &ast.ImportFrom{
						Module: "pkg",
						Names: []*ast.Node{
							{
								Node: &ast.Node_Alias{
									Alias: &ast.Alias{
										Name: "foo",
									},
								},
							},
							{
								Node: &ast.Node_Alias{
									Alias: &ast.Alias{
										Name: "bar",
									},
								},
							},
						},
					},
				},
			},
			Expected: `from pkg import foo, bar`,
		},
		"string-escape": {
			Node:     assign("X", poet.Constant(`say "hi" a\b`)),
			Expected: `X = "say \"hi\" a\\b"`,
		},
		"string-escape-multiline": {
			Node:     assign("X", poet.Constant("a\\b\n\"\"\"q\"")),
			Expected: `X = """a\\b` + "\n" + `\"""q\""""`,
		},
		"class-docstring-multiline": {
			Node: poet.Node(&ast.ClassDef{
				Name: "Foo",
				Body: []*ast.Node{
					poet.Expr(poet.Constant("first line\nsecond \"line\"")),
					annAssign("bar", poet.Name("int"), ""),
				},
			}),
			Expected: `
class Foo:
    """first line
second "line\""""
    bar: int
`,
		},
		"function-docstring": {
			Node: poet.Node(&ast.FunctionDef{
				Name: "f",
				Args: &ast.Arguments{Args: []*ast.Arg{{Arg: "self"}}},
				Body: []*ast.Node{
					poet.Expr(poet.Constant("Fetch an author.")),
					poet.Return(poet.Constant(1)),
				},
			}),
			Expected: `
def f(self):
    """Fetch an author."""
    return 1
`,
		},
		"class-empty": {
			Node: poet.Node(&ast.ClassDef{Name: "Params"}),
			Expected: `
class Params:
    pass
`,
		},
		"ann-assign-multiline-comment": {
			Node: poet.Node(&ast.ClassDef{
				Name: "Foo",
				Body: []*ast.Node{
					annAssign("bar", poet.Name("int"), "first line\nsecond line"),
				},
			}),
			Expected: `
class Foo:
    # first line
    # second line
    bar: int
`,
		},
		"list-comp": {
			Node: poet.ListComp(
				dict(poet.Constant("p1"), poet.Attribute(poet.Name("a"), "x")),
				poet.Name("a"),
				poet.Name("arg"),
			),
			Expected: `[{"p1": a.x} for a in arg]`,
		},
		"with": {
			Node: poet.With(
				poet.Node(&ast.Call{
					Func: poet.Attribute(poet.Name("errors"), "_wrap_errors"),
					Args: []*ast.Node{poet.Constant("q")},
				}),
				poet.Return(poet.Constant(nil)),
			),
			Expected: `
with errors._wrap_errors("q"):
    return None
`,
		},
		"bin-op": {
			Node:     poet.BitOr(poet.Subscript("list", poet.Name("int")), poet.Constant(nil)),
			Expected: `list[int] | None`,
		},
		"subscript-tuple": {
			Node:     poet.Subscript("Union", poet.Name("A"), poet.Name("B")),
			Expected: `Union[A, B]`,
		},
		"function-ellipsis": {
			Node: poet.Node(&ast.FunctionDef{
				Name:    "f",
				Args:    &ast.Arguments{Args: []*ast.Arg{{Arg: "self"}}},
				Body:    []*ast.Node{poet.Expr(poet.Ellipsis())},
				Returns: poet.Name("int"),
			}),
			Expected: `def f(self) -> int: ...`,
		},
		"class-type-params": {
			Node: poet.Node(&ast.ClassDef{
				Name: "Q",
				TypeParams: []*ast.TypeVar{
					{Name: "T", Bound: poet.BitOr(poet.Name("A"), poet.Name("B"))},
				},
				Body: []*ast.Node{annAssign("_conn", poet.Name("T"), "")},
			}),
			Expected: `
class Q[T: A | B]:
    _conn: T
`,
		},
		"if-not": {
			Node: poet.Node(&ast.If{
				Test: poet.Not(poet.Name("arg")),
				Body: []*ast.Node{poet.Return(poet.Constant(0))},
			}),
			Expected: `
if not arg:
    return 0
`,
		},
		"if-else": {
			Node: poet.Node(&ast.If{
				Test: poet.Name("row"),
				Body: []*ast.Node{poet.Expr(poet.Yield(poet.Name("row")))},
				OrElse: []*ast.Node{
					poet.Expr(poet.Yield(poet.Constant(nil))),
				},
			}),
			Expected: `
if row:
    yield row
else:
    yield None
`,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			result := Print(tc.Node, Options{})
			if diff := cmp.Diff(strings.TrimSpace(tc.Expected), strings.TrimSpace(string(result.Python))); diff != "" {
				t.Errorf("print mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func assign(target string, value *ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Assign{
			Assign: &ast.Assign{
				Targets: []*ast.Node{poet.Name(target)},
				Value:   value,
			},
		},
	}
}

func annAssign(target string, annotation *ast.Node, comment string) *ast.Node {
	return poet.Node(&ast.AnnAssign{
		Target:     &ast.Name{Id: target},
		Annotation: annotation,
		Comment:    comment,
	})
}

func dict(key, value *ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Dict{
			Dict: &ast.Dict{
				Keys:   []*ast.Node{key},
				Values: []*ast.Node{value},
			},
		},
	}
}
