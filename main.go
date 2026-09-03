// Package main is the entry point of the Revenant CLI.
//
// Cobra convention: keep main.go tiny. All commands live under ./cmd.
// When you add `revenant init` or `revenant report` later, you still only
// call cmd.Execute() from here — you never put command logic in main.
package main

import "github.com/pawan-bisht/revenant/cmd"

func main() {
	cmd.Execute()
}
