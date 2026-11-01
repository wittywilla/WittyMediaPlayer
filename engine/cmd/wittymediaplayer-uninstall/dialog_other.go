//go:build !windows

package main

import "fmt"

// The player is only built for Windows: this lets the program compile and be tested anywhere.

func show(_, text string) { fmt.Println(text) }

func confirm(_, text string) bool {
	fmt.Println(text)
	return false
}

func attachConsole() {}
