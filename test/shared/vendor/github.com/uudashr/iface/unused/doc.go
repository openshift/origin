// Package unused defines an Analyzer that detects interfaces which are not
// used anywhere in the same package where they are defined.
//
// Exported interfaces are reported without a suggested fix, since they may be
// used by other packages. Only unexported interfaces, which cannot be
// referenced outside their package, get a suggested fix that removes the
// declaration.
package unused
