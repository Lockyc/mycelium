package main

import (
	"fmt"
	"os"

	"github.com/lockyc/mycelium"
)

func usage() string {
	return `myco — the Mycelium ecosystem graph tool

usage: myco <command> [flags]

commands:
  scan       scan repo roots, emit a node manifest
  build      merge manifests + overlay into MAP.md + graph.json
  serve      build, then serve the map + graph over HTTP
  audit      check the graph for orphans, dangling edges, staleness
  validate   lint a single mycelium.toml against the schema
  query      query the graph (capabilities, components, used-by, search) — no jq
  version    print the myco version (also --version, -v)
`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage())
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println("myco " + mycelium.Version)
		return
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
	case "query":
		err = runQuery(os.Args[2:])
	default:
		fmt.Fprint(os.Stderr, usage())
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
