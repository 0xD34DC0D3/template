package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Best-effort: vars already set in the environment take precedence.
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = runServe(os.Args[2:])
	case "migrate":
		err = runMigrate(os.Args[2:])
	case "doctor":
		err = runDoctor(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "appname: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "appname:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`appname — a 0xD34DC0D3 app

Usage:
  appname serve
  appname migrate up | down | status
  appname doctor`)
}
