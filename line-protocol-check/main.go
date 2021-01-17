package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/influxdata/line-protocol-corpus/lpcorpus"
	"github.com/influxdata/line-protocol/influxdata"
	"github.com/rogpeppe/misc/ebnf2regexp"
	"github.com/rogpeppe/misc/ebnf2regexp/ebnf"
)

func main() {
	f, err := os.Open("line-protocol.ebnf")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	g, err := ebnf.Parse(f.Name(), f)
	if err != nil {
		log.Fatal(err)
	}
	reStr, err := ebnf2regexp.Translate(g, "lines")
	if err != nil {
		log.Fatal(err)
	}
	re := regexp.MustCompile(reStr)
	if err := runSanityChecks(re); err != nil {
		log.Fatalf("sanity checks failed: %v", err)
	}
	corpus, err := lpcorpus.ReadResults("/home/rogpeppe/src/influx/line-protocol-corpus")
	if err != nil {
		log.Fatal(err)
	}
	names := make([]string, 0, len(corpus.Decode))
	for name := range corpus.Decode {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		testCase := corpus.Decode[name]
		inp := testCase.Input
		if !utf8.Valid(inp.Text) {
			continue
		}
		text := translate(inp.Text)
		isValid := isValidLineProtocol(text)
		matched := re.Match(text)
		if isValid == matched {
			continue
		}
		if matched && !isValid && hasOutOfRangeError(testCase) {
			// The grammar doesn't cover out of range values.
			continue
		}
		fmt.Printf("test %s: mismatch (got %v want %v) on %q\n", name, matched, isValid, text)
	}
}

// translate translates characters that were previously allowed by the
// old syntax into appropriate characters for the new syntax
// so that we can usefully compare match results without spurious
// failures.
func translate(text []byte) []byte {
	text1 := make([]byte, len(text))
	for i, b := range text {
		switch {
		case wasSpace[b]:
			text1[i] = ' '
		case wasNonSpace[b]:
			text1[i] = 'X'
		default:
			text1[i] = b
		}
	}
	return text1
}

var wasSpace = [256]bool{
	'\t': true,
	'\f': true,
}

var wasNonSpace = func() [256]bool {
	var r [256]bool
	for i := 0; i < 31; i++ {
		r[i] = true
	}
	r[127] = true
	r['\t'] = false
	r['\r'] = false
	return r
}()

func hasOutOfRangeError(res *lpcorpus.DecodeResults) bool {
	for _, out := range res.Output {
		if strings.Contains(out.Error, "out of range") {
			return true
		}
	}
	return false
}

var sanityChecks = []struct {
	text   string
	expect bool
}{{
	text:   "foo v=-32.56e65 4343",
	expect: true,
}, {
	text:   `cpu,host=serverA,region=us-east value="{Hello\"{\,}\" World}" 1000000000`,
	expect: true,
}, {
	text:   `cpu value="test\\" 1000000000`,
	expect: true,
}, {
	text:   "a\\\nb f=1",
	expect: false,
}, {
	text:   "ab f\\\nb=1",
	expect: false,
}, {
	text:   "ab,t\\\nx=p f=1",
	expect: false,
}, {
	text:   "ab,t=p\\\nq f=1",
	expect: false,
}}

func runSanityChecks(re *regexp.Regexp) error {
	for _, test := range sanityChecks {
		matched := re.MatchString(test.text)
		isValidLineProtocol([]byte(test.text))
		if matched != test.expect {
			return fmt.Errorf("failed on `%s`", test.text)
		}
	}
	return nil
}

func isValidLineProtocol(s []byte) bool {
	tok := influxdata.NewTokenizerWithBytes(s)

	for tok.Next() {
		if _, err := tok.Measurement(); err != nil {
			return false
		}
		for {
			key, _, err := tok.NextTag()
			if err != nil {
				return false
			}
			if key == nil {
				break
			}
		}
		for {
			key, _, err := tok.NextField()
			if err != nil {
				return false
			}
			if key == nil {
				break
			}
		}
		_, err := tok.Time(influxdata.Nanosecond, time.Time{})
		if err != nil {
			return false
		}
	}
	if tok.Err() != nil {
		return false
	}
	return true
}
