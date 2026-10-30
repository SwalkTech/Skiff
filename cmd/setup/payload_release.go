//go:build installer

package main

import _ "embed"

//go:embed Skiff.exe
var payload []byte
