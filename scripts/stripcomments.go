package main

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <file.go>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error in %s: %v\n", filename, err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	cfg := &printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}

	if err := cfg.Fprint(&buf, fset, file); err != nil {
		fmt.Fprintf(os.Stderr, "print error: %v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat error: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(filename, buf.Bytes(), info.Mode()); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
}
