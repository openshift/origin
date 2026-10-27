// Package unusedmethod defines an Analyzer that detects interface methods which
// are never used anywhere in the same package where they are defined.
//
// A method is considered used only when it is invoked or referenced through a
// value of the interface type. Merely implementing the interface — for example,
// assigning a concrete type to the interface, or calling the method directly on
// the concrete type — does not count as a use.
//
// Exported methods are reported without a suggested fix, since they may be used
// by other packages — even when declared on an unexported interface that is
// reachable through an exported API. Only unexported methods, which cannot be
// referenced outside their package, get a suggested fix that removes the method.
package unusedmethod
