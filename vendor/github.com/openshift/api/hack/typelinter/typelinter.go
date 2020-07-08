package main

import (
	"flag"
	"fmt"
	"go/types"
	"os"
	"regexp"

	"golang.org/x/tools/go/packages"
)

type stringListVar map[string]struct{}

func (sl stringListVar) String() string {
	var keys []string
	for k := range sl {
		keys = append(keys, k)
	}
	return fmt.Sprintf("%v", keys)
}

func (sl *stringListVar) Set(value string) error {
	if *sl == nil {
		*sl = map[string]struct{}{}
	}
	(*sl)[value] = struct{}{}
	return nil
}

var (
	excludedFields stringListVar
	typesWhitelist string

	typesWhitelistRegexp *regexp.Regexp
)

// validStruct checks whether the structure s uses only the whitelisted types.
// structName is the name of s in format `example.com/package.TypeName`.
func validStruct(structName string, s *types.Struct) bool {
	valid := true
	for i := 0; i < s.NumFields(); i++ {
		field := s.Field(i)
		fieldName := fmt.Sprintf("%s:%s", structName, field.Name())
		if _, ok := excludedFields[fieldName]; ok {
			continue
		}
		typ := field.Type().String()
		if !typesWhitelistRegexp.MatchString(typ) {
			fmt.Fprintf(os.Stderr, "%s: type %s is not allowed to be used\n", fieldName, typ)
			valid = false
		}
	}
	return valid
}

// validPackage checks whether pkg's exported structures use only the
// whitelisted types.
func validPackage(pkg *packages.Package) bool {
	valid := true
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		if typeName, ok := obj.(*types.TypeName); ok {
			typ := typeName.Type().Underlying()
			if s, ok := typ.(*types.Struct); ok {
				structName := fmt.Sprintf("%s.%s", pkg.PkgPath, typeName.Name())
				if !validStruct(structName, s) {
					valid = false
				}
			}
		}
	}
	return valid
}

func main() {
	flag.Var(&excludedFields, "excluded", "exclude the field from being checked (e.g. -excluded=github.com/openshift/api/image/dockerpre012.ImagePre012:Created), can be used multiple times")
	flag.StringVar(&typesWhitelist, "whitelist", "", "regular expression that specifies allowed types")
	flag.Parse()

	var err error
	typesWhitelistRegexp, err = regexp.Compile(typesWhitelist)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to compile whitelist regexp: %v\n", err)
		os.Exit(1)
	}

	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedImports | packages.NeedTypes}
	pkgs, err := packages.Load(cfg, flag.Args()...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load packages: %v\n", err)
		os.Exit(1)
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}

	ok := true
	for _, pkg := range pkgs {
		if !validPackage(pkg) {
			ok = false
		}
	}
	if !ok {
		os.Exit(1)
	}
}
