package ebnf2regexp

import (
	"regexp/syntax"
	"testing"

	qt "github.com/frankban/quicktest"
)

var orTests = []struct {
	exprs  []string
	expect string
}{{
	exprs:  []string{`[ab]`, `[bc]`},
	expect: `[a-c]`,
}, {
	exprs:  []string{`[^ab]`, `[^bc]`},
	expect: `[^b]`,
}, {
	exprs:  []string{`[^0-9]`, `[23]`},
	expect: `[^0-14-9]`,
}, {
	exprs:  []string{`[0-9]`, `\d`},
	expect: `[0-9]`,
}}

func TestOr(t *testing.T) {
	c := qt.New(t)
	for _, test := range orTests {
		c.Run(test.expect, func(c *qt.C) {
			es := make([]*syntax.Regexp, len(test.exprs))
			for i, e := range test.exprs {
				re, err := syntax.Parse(e, syntax.ClassNL|syntax.PerlX)
				c.Assert(err, qt.IsNil)
				es[i] = re
			}
			e := or(es)
			c.Assert(e.String(), qt.Equals, test.expect)
		})
	}
}
