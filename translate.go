package ebnf2regexp

import (
	"fmt"
	"reflect"
	"regexp/syntax"
	"unicode"

	"github.com/rogpeppe/misc/ebnf2regexp/ebnf"
)

// Line-protocol syntax as a regular expression.

func Translate(g ebnf.Grammar, start string) (_ string, err error) {
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if e := e.(*cvtError); e != nil {
			err = e.error
			return
		}
		panic(e)
	}()
	//if err := ebnf.Verify(g, start); err != nil {
	//	return "", fmt.Errorf("grammar verification failed: %v", err)
	//}
	return ebnfProd2regexp(g, start).String(), nil
}

type cvtError struct {
	error
}

func ebnfProd2regexp(g ebnf.Grammar, prod string) *syntax.Regexp {
	p := g[prod]
	if p == nil {
		fatalf("start production %q not found", prod)
	}
	e := ebnfExpr(g, p.Expr)
	return &syntax.Regexp{
		Op: syntax.OpConcat,
		Sub: []*syntax.Regexp{{
			Op: syntax.OpBeginText,
		}, e, {
			Op: syntax.OpEndText,
		}},
	}
}

func ebnfExpr(g ebnf.Grammar, e ebnf.Expression) *syntax.Regexp {
	switch e := e.(type) {
	case *ebnf.Range:
		start := []rune(e.Begin.String)
		end := []rune(e.End.String)
		if len(start) != 1 || len(end) != 1 {
			fatalf("range with limits that are not single characters")
		}
		if start[0] == 0 && end[0] == unicode.MaxRune {
			return &syntax.Regexp{
				Op: syntax.OpAnyChar,
			}
		}
		return &syntax.Regexp{
			Op:   syntax.OpCharClass,
			Rune: []rune{start[0], end[0]},
		}
	case *ebnf.Repetition:
		return &syntax.Regexp{
			Op: syntax.OpStar,
			Sub: []*syntax.Regexp{
				ebnfExpr(g, e.Body),
			},
		}
	case *ebnf.Production:
		return ebnfExpr(g, e.Expr)
	case ebnf.Sequence:
		es := ebnfExprs(g, e)
		for i := 0; i < len(es)-1; i++ {
			if es[i+1].Op == syntax.OpStar && reflect.DeepEqual(es[i], es[i+1].Sub[0]) {
				// We've found an instance of `foo {foo}` which can be turned into a one-or-more
				// operator.
				ne := make([]expr, len(es)-1)
				copy(ne, es[:i])
				ne[i] = &syntax.Regexp{
					Op:  syntax.OpPlus,
					Sub: es[i+1].Sub,
				}
				copy(ne[i+1:], es[i+2:])
				es = ne
			}
		}
		return &syntax.Regexp{
			Op:  syntax.OpConcat,
			Sub: es,
		}
	case *ebnf.Option:
		return &syntax.Regexp{
			Op: syntax.OpQuest,
			Sub: []*syntax.Regexp{
				ebnfExpr(g, e.Body),
			},
		}
	case *ebnf.Name:
		e1 := g[e.String]
		if e1 == nil {
			fatalf("production name %q not found", e.String)
		}
		return ebnfExpr(g, e1)
	case *ebnf.Group:
		return ebnfExpr(g, e.Body)
	case ebnf.Alternative:
		return or(ebnfExprs(g, e))
	case *ebnf.Token:
		return &syntax.Regexp{
			Op:   syntax.OpLiteral,
			Rune: []rune(e.String),
		}
	case *ebnf.Complement:
		return not(ebnfExpr(g, e.Body))
	default:
		panic(fmt.Errorf("unknown ebnf node %#v", e))
	}
}

func ebnfExprs(g ebnf.Grammar, es []ebnf.Expression) []*syntax.Regexp {
	sub := make([]*syntax.Regexp, len(es))
	for i, e := range es {
		sub[i] = ebnfExpr(g, e)
	}
	return sub
}

type expr = *syntax.Regexp

func suffixOp(e expr, op syntax.Op) expr {
	return &syntax.Regexp{
		Op:  op,
		Sub: []*syntax.Regexp{e},
	}
}

func oneOrMore(e expr) expr {
	return suffixOp(e, syntax.OpPlus)
}

func or(es []expr) expr {
	// Assume until proved otherwise that the subexpressions are single characters,
	// so the result is a single character, and build up the union inside rs.
	var rs []rune
	for _, e := range es {
		if e.Op == syntax.OpCharClass {
			rs = unionRange(rs, e.Rune)
			continue
		}
		if e.Op == syntax.OpLiteral && len(e.Rune) == 1 {
			lit := []rune(e.Rune)
			rs = unionRange(rs, []rune{lit[0], lit[0]})
			continue
		}
		// It's not a single character, so just use alternation.
		return &syntax.Regexp{
			Op:  syntax.OpAlternate,
			Sub: es,
		}
	}
	return &syntax.Regexp{
		Op:   syntax.OpCharClass,
		Rune: rs,
	}
}

func unionRange(rs0, rs1 []rune) []rune {
	in0, in1 := false, false
	in := false
	var rsu []rune
	for len(rs0) != 0 || len(rs1) != 0 {
		r0 := unicode.MaxRune + 1
		if len(rs0) > 0 {
			r0 = rs0[0]
		}
		r1 := unicode.MaxRune + 1
		if len(rs1) > 0 {
			r1 = rs1[0]
		}
		r := rune(0)
		if r0 <= r1 {
			r = r0
			in0 = !in0
			rs0 = rs0[1:]
		}
		if r1 <= r0 {
			r = r1
			in1 = !in1
			rs1 = rs1[1:]
		}
		if (in0 || in1) != in {
			rsu = append(rsu, r)
			in = in0 || in1
		}
	}
	return rsu
}

func not(e expr) expr {
	switch e.Op {
	case syntax.OpCharClass:
		if e.Rune[0] == 0 && e.Rune[len(e.Rune)-1] == unicode.MaxRune {
			fatalf("cannot negate negated class %q", e)
		}
		runes := []rune{0}
		for i := 0; i < len(e.Rune); i += 2 {
			runes = append(runes, e.Rune[i]-1)
			runes = append(runes, e.Rune[i+1]+1)
		}
		runes = append(runes, unicode.MaxRune)
		return &syntax.Regexp{
			Op:   syntax.OpCharClass,
			Rune: runes,
		}
	case syntax.OpLiteral:
		if len(e.Rune) != 1 {
			fatalf("cannot negate a literal %q containing more than one char", e)
		}
		return &syntax.Regexp{
			Op:   syntax.OpCharClass,
			Rune: []rune{0, e.Rune[0] - 1, e.Rune[0] + 1, unicode.MaxRune},
		}
	default:
		fatalf("cannot negate %q", e)
	}
	panic("unreachable")
}

func fatalf(f string, a ...interface{}) {
	panic(&cvtError{fmt.Errorf(f, a...)})
}
