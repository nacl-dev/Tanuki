package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nacl-dev/tanuki/internal/downloader"
)

func main() {
	modulesDir := flag.String("modules-dir", "", "Path to an HDoujinDownloader modules/lua directory")
	outPath := flag.String("out", "", "Optional path for the generated JSON audit report")
	flag.Parse()

	if *modulesDir == "" {
		fmt.Fprintln(os.Stderr, "missing required -modules-dir")
		os.Exit(2)
	}

	report, err := downloader.AuditHDoujinModulesDir(*modulesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit modules: %v\n", err)
		os.Exit(1)
	}

	body, err := report.JSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
		os.Exit(1)
	}

	if *outPath != "" {
		if err := os.WriteFile(*outPath, body, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote HDoujin audit report to %s\n", *outPath)
		return
	}

	os.Stdout.Write(body) //nolint:errcheck
}
