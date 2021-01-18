package ebnf2regexp

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/ebnf2regexp/ebnf"
)

var translateTests = []struct {
	testName string
	ebnf     string
	start    string
	expect   string
}{{
	testName: "simple-literal",
	ebnf: `
	start = "hello" .
	`,
	start:  "start",
	expect: `\Ahello\z`,
}, {
	testName: "simple-literal",
	ebnf: `
	start = "hello" | "goodbye" .
	`,
	start:  "start",
	expect: `\A(?:hello|goodbye)\z`,
}, {
	testName: "simple-literal",
	ebnf: `
	start = "a" | "b" | "c" .
	`,
	start:  "start",
	expect: `\A[abc]\z`,
}, {
	testName: "simple-literal",
	ebnf: `
	start = "a" | "b" | "c" .
	`,
	start:  "start",
	expect: `\A[abc]\z`,
}, {
	testName: "ellipses",
	ebnf: `
	start = "a"…"f" | "0"…"9" .
	`,
	start:  "start",
	expect: `\A[0-9a-f]\z`,
}, {
	testName: "not",
	ebnf: `
	start = not hex .
	hex = "a"…"f" | "0"…"9" .
	`,
	start:  "start",
	expect: `\A[^0-9a-f]\z`,
}}

func TestTranslate(t *testing.T) {
	c := qt.New(t)
	for _, test := range translateTests {
		c.Run(test.testName, func(c *qt.C) {
			g, err := ebnf.Parse("test", strings.NewReader(test.ebnf))
			c.Assert(err, qt.IsNil)
			r, err := Translate(g, test.start)
			c.Assert(err, qt.IsNil)
			c.Assert(r, qt.Equals, test.expect)
		})
	}
}
