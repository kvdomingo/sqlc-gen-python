package python

// Hard keywords from keyword.kwlist in Python 3.12. Soft keywords (match,
// case, type, _) are valid identifiers and are left alone.
var pyKeywords = map[string]struct{}{
	"False": {}, "None": {}, "True": {}, "and": {}, "as": {}, "assert": {},
	"async": {}, "await": {}, "break": {}, "class": {}, "continue": {},
	"def": {}, "del": {}, "elif": {}, "else": {}, "except": {}, "finally": {},
	"for": {}, "from": {}, "global": {}, "if": {}, "import": {}, "in": {},
	"is": {}, "lambda": {}, "nonlocal": {}, "not": {}, "or": {}, "pass": {},
	"raise": {}, "return": {}, "try": {}, "while": {}, "with": {}, "yield": {},
}

func escapeKeyword(name string) string {
	if _, ok := pyKeywords[name]; ok {
		return name + "_"
	}
	return name
}

// pyIdent is the single source of generated Python names. A rename entry for
// dbName wins over transform; the result is then keyword-escaped.
func pyIdent(dbName string, conf Config, transform func(string) string) string {
	if r, ok := conf.Rename[dbName]; ok && r != "" {
		return escapeKeyword(r)
	}
	if transform != nil {
		dbName = transform(dbName)
	}
	return escapeKeyword(dbName)
}
