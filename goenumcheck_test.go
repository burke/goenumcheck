package goenumcheck

import (
	"go/build"
	"go/parser"
	"io/ioutil"
	"os"
	"testing"

	"golang.org/x/tools/go/loader"
	"honnef.co/go/lint"
)

var testSrc = `package testdata

type myType int

const (
	my1 myType = iota
	my2
	my3
)

func arbitrary(my myType) {
	switch my {
	case my1, my2, my3:
	}

	switch my {
	case my1, my3: // missing my2
	}

	switch my {
	case my1, my3: // missing my2
	default: // but no error because default
	}

	switch my {
	case myType(1): // missing all three
	}

	switch my {
	default: // missing all three, but no error
	}

	switch my {
	case my1:
	case my2, my3: // all good
	}
}
`

var testExpected = []string{
	"testdata.go:16:2: uncovered cases for myType enum switch\n\t- my2 (EC1000)",
	"testdata.go:25:2: uncovered cases for myType enum switch\n\t- my1\n\t- my2\n\t- my3 (EC1000)",
}

func TestAll(t *testing.T) {
	ctx := build.Default
	conf2 := &loader.Config{
		Build:      &ctx,
		ParserMode: parser.ParseComments,
	}

	tf, err := ioutil.TempFile(os.TempDir(), "XXXXXX")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tf.Name())
	path := tf.Name()
	if err := ioutil.WriteFile(path, []byte(testSrc), 0644); err != nil {
		t.Fatal(err)
	}

	conf2.CreateFromFilenames("adhoc", path)

	lprog, err := conf2.Load()
	if err != nil {
		t.Fatal(err) // type error
	}

	l := &lint.Linter{
		Checker: NewChecker(),
	}

	problems := l.Lint(lprog)
	if len(problems) != 1 {
		t.Error("expected 1 problem")
	}
	for k, v := range problems {
		if k != "adhoc" {
			t.Error("expected package name to be adhoc")
		}
		if len(v) != 2 {
			t.Error("expected 2 problems")
		}
		for i, prob := range v {
			prob.Position.Filename = "testdata.go"
			out := prob.Position.String() + ": " + prob.String()
			if out != testExpected[i] {
				t.Error("wrong output:\n" + out + "\nwanted:\n" + testExpected[i])
			}
		}
	}
}
