package main

import (
	"os"

	"github.com/burke/goenumcheck"
	"honnef.co/go/lint/lintutil"
)

func main() {
	lintutil.ProcessArgs("goenumcheck", goenumcheck.NewChecker(), os.Args[1:])
}
