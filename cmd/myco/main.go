package main

import (
	"fmt"
	"os"
)

func usage() string {
	return `myco — the Mycelium catalog tool

usage: myco <command> [flags]

commands:
  scan       scan repo roots, emit a node manifest
  build      merge manifests + overlay into CATALOG.md + catalog.json
  serve      build, then serve the catalog over HTTP
  audit      check the catalog for orphans, dangling edges, staleness
  validate   lint a single catalog.toml against the schema
`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage())
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "scan":
		err = runScan(os.Args[2:])
	case "build":
		err = runBuild(os.Args[2:])
	case "serve":
		err = runServe(os.Args[2:])
	case "audit":
		err = runAudit(os.Args[2:])
	case "validate":
		err = runValidate(os.Args[2:])
	default:
		fmt.Fprint(os.Stderr, usage())
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
