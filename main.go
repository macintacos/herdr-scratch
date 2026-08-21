// Command herdr-scratch is the scratch-shell popup plugin for herdr.
//
// The constraint that shapes it: a herdr popup receives every byte of terminal
// input until its command exits, so no herdr keybinding can reach one. Closing
// therefore comes either from outside, before the popup exists (toggle), or from
// inside the popup itself (dismiss).
package main

import "github.com/macintacos/herdr-scratch/cmd"

func main() {
	cmd.Execute()
}
