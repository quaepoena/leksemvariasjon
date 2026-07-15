// Package types holds an interface type to allow for separation into packages.
// Despite being recognized as bad Go-design, this seems a reasonable compromise
// to prevent having all the code in package main, which would be a consequence
// of the resumption capability of the program.
package types

type WorkflowStage interface {
	Finished() bool
	Run(*Args, *Conf) error
}
