//go:build js && wasm

// Command wasm is the web build of be. It registers globalThis.be for the page
// in web/ and then waits forever, serving calls from JavaScript.
package main

import "github.com/boringemployee/be/internal/adapter/wasm"

func main() {
	wasm.Register()
	select {}
}
