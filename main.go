package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp/syntax"
	"unicode"

	"github.com/rogpeppe/misc/ebnf2regexp/ebnf"
)

// Line-protocol syntax as a regular expression.

//
//var (
//	lines           = seq("^", line, zeroOrMore(seq(newline_char, line)), "$")
//	whitespace_char = or(lit(" "), lit("\r"))
//	newline_char    = lit("\n")
//	whitespace      = oneOrMore(whitespace_char)
//	line            = seq(zeroOrMore(whitespace_char), opt(or(point, comment)))
//	point           = seq(measurement, zeroOrMore(seq(",", tag)), whitespace, field, zeroOrMore(lit(",")+field), opt(seq(whitespace, timestamp)), zeroOrMore(whitespace_char))
//	comment         = seq(lit("#"), not(newline_char))
//
//	measurement              = zeroOrMore(or(measurement_regular_char, measurement_escape_seq))
//	measurement_regular_char = not(or(whitespace_char, lit(`\`), lit(",")))
//	measurement_escape_seq   = seq(oneOrMore(lit(`\`)), lit(","))
//
//	tag              = seq(key, lit("="), tagval)
//	key              = zeroOrMore(or(key_regular_char, key_escape_seq))
//	key_regular_char = not(or(whitespace_char, newline_char, lit(`\`), lit(","), lit("=")))
//	key_escape_seq   = seq(oneOrMore(lit(`\`)), or(lit(","), lit("=")))
//
//	tagval              = zeroOrMore(or(tagval_regular_char, tagval_escape_seq))
//	tagval_regular_char = not(or(whitespace_char, newline_char, lit(","), lit("="), lit(" ")))
//	tagval_escape_seq   = seq(oneOrMore(lit(`\`)), or(lit(" "), lit(","), lit("=")))
//
//	field          = seq(key, "=", fieldval)
//	fieldval       = or(boolfield, stringfield, intfield, uintfield, floatfield)
//	decimal_digit  = or(lit("0"), lit("1"), lit("2"), lit("3"), lit("4"), lit("5"), lit("6"), lit("7"), lit("8"), lit("9"))
//	decimal_digits = oneOrMore(decimal_digit)
//
//	boolfield  = or(lit("t"), lit("T"), lit("true"), lit("True"), lit("TRUE"), lit("f"), lit("F"), lit("false"), lit("False"), lit("FALSE"))
//	intfield   = seq(opt(lit("-")), decimal_digits, lit("i"))
//	uintfield  = seq(decimal_digits, lit("u"))
//	floatfield = or(
//		seq(decimal_digits, lit("."), opt(decimal_digits), opt(decimal_exponent)),
//		seq(decimal_digits, decimal_exponent),
//		seq(lit("."), decimal_digits, opt(decimal_exponent)),
//	)
//	decimal_exponent = seq(or(lit("e"), lit("E")), opt(or(lit("+"), lit("-"))), decimal_digits)
//	stringfield      = seq(lit(`"`), zeroOrMore(or(not(or(lit(`"`), lit(`\`))), lit(`\\`), lit(`\"`), lit(`\n`), lit(`\r`), lit(`\t`))), lit(`"`))
//	timestamp        = decimal_digits
//)

var (
	verbose = flag.Bool("v", true, "verbose mode")
	start   = flag.String("start", "start", "top level production")
)

func main() {
	flag.Parse()
	r := os.Stdin
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		r = f
	}
	g, err := ebnf.Parse(r.Name(), r)
	if err != nil {
		log.Fatal(err)
	}
	if err := ebnf.Verify(g, *start); err != nil {
		log.Fatal(err)
	}
	re := ebnf2regexp(g, g[*start].Expr)
	fmt.Printf("%v\n", re)
	//	s := strings.ReplaceAll(string(lines), "\n", "\\n")
	//	if *verbose {
	//		fmt.Println(s)
	//	}
	//	re := regexp.MustCompile(s)
	//	if flag.NArg() > 0 {
	//		fmt.Println(re.MatchString(flag.Arg(0)))
	//	}
}

func ebnf2regexp(g ebnf.Grammar, e ebnf.Expression) *syntax.Regexp {
	fmt.Printf("ebnf2regexp %T {\n", e)
	defer fmt.Println("}")
	switch e := e.(type) {
	case *ebnf.Range:
		start := []rune(e.Begin.String)
		end := []rune(e.End.String)
		if len(start) != 1 || len(end) != 1 {
			panic("range with limits that are not single charaxters")
		}
		return &syntax.Regexp{
			Op:   syntax.OpCharClass,
			Rune: []rune{start[0], end[0]},
		}
	case *ebnf.Repetition:
		return &syntax.Regexp{
			Op: syntax.OpStar,
			Sub: []*syntax.Regexp{
				ebnf2regexp(g, e.Body),
			},
		}
	case *ebnf.Production:
		return ebnf2regexp(g, e.Expr)
	case ebnf.Sequence:
		return &syntax.Regexp{
			Op:  syntax.OpConcat,
			Sub: ebnfExprs(g, e),
		}
	case *ebnf.Option:
		return &syntax.Regexp{
			Op: syntax.OpQuest,
			Sub: []*syntax.Regexp{
				ebnf2regexp(g, e.Body),
			},
		}
	case *ebnf.Name:
		e1 := g[e.String]
		if e1 == nil {
			panic("name not found")
		}
		return ebnf2regexp(g, e1)
	case *ebnf.Group:
		return ebnf2regexp(g, e.Body)
	case ebnf.Alternative:
		return or(ebnfExprs(g, e)...)
	case *ebnf.Token:
		return &syntax.Regexp{
			Op:   syntax.OpLiteral,
			Rune: []rune(e.String),
		}
	case *ebnf.Complement:
		return not(ebnf2regexp(g, e.Body))
	default:
		panic(fmt.Errorf("unknown ebnf node %#v", e))
	}
}

func ebnfExprs(g ebnf.Grammar, es []ebnf.Expression) []*syntax.Regexp {
	sub := make([]*syntax.Regexp, len(es))
	for i, e := range es {
		sub[i] = ebnf2regexp(g, e)
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

func or(es ...expr) expr {
	runeset := make(map[rune]bool)
outer:
	for _, e := range es {
		switch e.Op {
		case syntax.OpLiteral:
			if len(e.Rune) != 1 {
				runeset = nil
				break outer
			}
			runeset[e.Rune[0]] = true
		case syntax.OpCharClass:
			for i := 0; i < len(e.Rune); i += 2 {
				for j := e.Rune[i]; j <= e.Rune[i+1]; j++ {
					runeset[j] = true
				}
			}
		default:
			runeset = nil
			break outer
		}
	}
	if runeset != nil {
		var runes []rune
		on := false

		for i := rune(0); i <= unicode.MaxRune+1; i++ {
			if runeset[i] {
				if on {
					continue
				}
				runes = append(runes, i)
				on = true
			} else {
				if !on {
					continue
				}
				runes = append(runes, i-1)
				on = false
			}
		}
		return &syntax.Regexp{
			Op:   syntax.OpCharClass,
			Rune: runes,
		}
	}
	return &syntax.Regexp{
		Op:  syntax.OpAlternate,
		Sub: es,
	}
}

func not(e expr) expr {
	switch e.Op {
	case syntax.OpCharClass:
		if e.Rune[0] == 0 {
			panic("cannot negate negated class")
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
			panic("cannot negate a literal containing more than one char")
		}
		return &syntax.Regexp{
			Op:   syntax.OpCharClass,
			Rune: []rune{0, e.Rune[0] - 1, e.Rune[0] + 1, unicode.MaxRune},
		}
	default:
		panic(fmt.Errorf("cannot negate %q", e))
	}
}
