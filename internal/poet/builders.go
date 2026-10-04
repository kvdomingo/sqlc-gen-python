package poet

import "github.com/sqlc-dev/sqlc-gen-python/internal/ast"

func Alias(name string) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Alias{
			Alias: &ast.Alias{
				Name: name,
			},
		},
	}
}

func Await(value *ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Await{
			Await: &ast.Await{
				Value: value,
			},
		},
	}
}

func Attribute(value *ast.Node, attr string) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Attribute{
			Attribute: &ast.Attribute{
				Value: value,
				Attr:  attr,
			},
		},
	}
}

func Comment(text string) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Comment{
			Comment: &ast.Comment{
				Text: text,
			},
		},
	}
}

func Expr(value *ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Expr{
			Expr: &ast.Expr{
				Value: value,
			},
		},
	}
}

func Is() *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Is{
			Is: &ast.Is{},
		},
	}
}

func Name(id string) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Name{
			Name: &ast.Name{Id: id},
		},
	}
}

func Return(value *ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Return{
			Return: &ast.Return{
				Value: value,
			},
		},
	}
}

func Yield(value *ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Yield{
			Yield: &ast.Yield{
				Value: value,
			},
		},
	}
}

func BitOr(left, right *ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_BinOp{
			BinOp: &ast.BinOp{
				Left:  left,
				Op:    &ast.Node{Node: &ast.Node_BitOr{BitOr: &ast.BitOr{}}},
				Right: right,
			},
		},
	}
}

func Ellipsis() *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Constant{
			Constant: &ast.Constant{
				Value: &ast.Constant_Ellipsis{Ellipsis: true},
			},
		},
	}
}

func ListComp(elt, target, iter *ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_ListComp{
			ListComp: &ast.ListComp{
				Elt: elt,
				Generators: []*ast.Comprehension{
					{Target: target, Iter: iter},
				},
			},
		},
	}
}

func Subscript(value string, slice ...*ast.Node) *ast.Node {
	s := slice[0]
	if len(slice) > 1 {
		s = Tuple(slice...)
	}
	return &ast.Node{
		Node: &ast.Node_Subscript{
			Subscript: &ast.Subscript{
				Value: &ast.Name{Id: value},
				Slice: s,
			},
		},
	}
}

func Tuple(elts ...*ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_Tuple{
			Tuple: &ast.Tuple{Elts: elts},
		},
	}
}

func With(contextExpr *ast.Node, body ...*ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_With{
			With: &ast.With{
				Items: []*ast.WithItem{{ContextExpr: contextExpr}},
				Body:  body,
			},
		},
	}
}

func Not(operand *ast.Node) *ast.Node {
	return &ast.Node{
		Node: &ast.Node_UnaryOp{
			UnaryOp: &ast.UnaryOp{
				Op:      &ast.Node{Node: &ast.Node_Not{Not: &ast.Not{}}},
				Operand: operand,
			},
		},
	}
}
