package astutils

import "go/ast"

// exitFuncChecker is a function type that checks whether a function call is an exit function.
type exitFuncChecker func(args []ast.Expr) bool

var alwaysTrue exitFuncChecker = func([]ast.Expr) bool { return true }

// exitFunctions is a map of std packages and functions that are considered as exit functions.
var exitFunctions = map[string]map[string]exitFuncChecker{
	"os":      {"Exit": alwaysTrue},
	"syscall": {"Exit": alwaysTrue},
	"log": {
		"Fatal":   alwaysTrue,
		"Fatalf":  alwaysTrue,
		"Fatalln": alwaysTrue,
		"Panic":   alwaysTrue,
		"Panicf":  alwaysTrue,
		"Panicln": alwaysTrue,
	},
	"flag": {
		"Parse": func([]ast.Expr) bool { return true },
		"NewFlagSet": func(args []ast.Expr) bool {
			if len(args) != 2 {
				return false
			}
			return IsPkgDotName(args[1], "flag", "ExitOnError")
		},
	},
}

// IsCallToExitFunction checks if the function call is a call to an exit function.
func IsCallToExitFunction(pkgName, functionName string, callArgs []ast.Expr) bool {
	m, ok := exitFunctions[pkgName]
	if !ok {
		return false
	}

	check, ok := m[functionName]
	if !ok {
		return false
	}

	return check(callArgs)
}
