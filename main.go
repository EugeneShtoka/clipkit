package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// errNoMatch is returned by runJSONMode when extraction finds nothing.
// The caller exits with code 1 (nothing found); errors exit 2+.
var errNoMatch = errors.New("no match")

func runJSONMode(r io.Reader, w io.Writer, field string, opts []Option, extraction bool) error {
	data, err := io.ReadAll(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: read error: %v\n", err)
		os.Exit(2)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: json parse: %v\n", err)
		os.Exit(2)
	}

	var value string
	if raw, ok := obj[field]; ok {
		if err := json.Unmarshal(raw, &value); err != nil {
			fmt.Fprintf(os.Stderr, "clipkit: field %q parse: %v\n", field, err)
			os.Exit(2)
		}
	}

	out, err := Parse(value, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: %v\n", err)
		os.Exit(2)
	}
	if out == "" {
		return errNoMatch
	}

	if extraction {
		fmt.Fprintln(w, out)
		return nil
	}

	outJSON, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: json marshal: %v\n", err)
		os.Exit(2)
	}
	obj[field] = outJSON

	result, err := json.Marshal(obj)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: json marshal: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintln(w, string(result))
	return nil
}

func run(r io.Reader, w io.Writer, opts []Option, jsonField string, extraction bool) error {
	if jsonField != "" {
		return runJSONMode(r, w, jsonField, opts, extraction)
	}

	scanner := bufio.NewScanner(r)
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	var lines []string
	for scanner.Scan() {
		out, err := Parse(scanner.Text(), opts...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "clipkit: %v\n", err)
			os.Exit(2)
		}
		lines = append(lines, out)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: read error: %v\n", err)
		os.Exit(2)
	}

	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	fmt.Fprint(bw, strings.Join(lines, "\n"))
	return nil
}

func main() {
	jsonField    := flag.String("json", "", "JSON field name: read JSON from stdin, transform the named field, write JSON to stdout")
	collapse     := flag.Bool("collapse", false, "Collapse internal whitespace runs to a single space")
	cutset       := flag.String("cutset", "", "Characters to trim from both ends")
	maxLen       := flag.Int("max-len", 0, "Truncate at word boundary to this many runes (0 = off)")
	stripTs      := flag.Bool("strip-timestamps", false, "Remove bracketed timestamps at start/end, e.g. [19:54:47]")
	stripBorders := flag.Bool("strip-borders", false, "Remove box-drawing border chars (│ ┃ ─ ━ |) from both ends")
	extractCode  := flag.Bool("extract-code", false, "Extract first 5–6 char alphanumeric code; exits 1 if none found")
	extractURL   := flag.Bool("extract-url", false, "Extract all URLs; exits 1 if none found")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: clipkit [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Reads lines from stdin, trims each one, writes to stdout.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  echo '  hello   world  ' | clipkit --collapse\n")
		fmt.Fprintf(os.Stderr, "  cat app.log | clipkit --strip-borders --collapse --strip-timestamps\n")
		fmt.Fprintf(os.Stderr, "  echo '{\"body\":\"OTP: 482910\"}' | clipkit --json body --extract-code\n")
	}

	flag.Parse()

	var opts []Option
	if *collapse      { opts = append(opts, WithCollapseSpaces()) }
	if *cutset != ""  { opts = append(opts, WithCutset(*cutset)) }
	if *maxLen > 0    { opts = append(opts, WithMaxLen(*maxLen)) }
	if *stripTs       { opts = append(opts, WithStripTimestamps()) }
	if *stripBorders  { opts = append(opts, WithStripBorders()) }
	if *extractCode   { opts = append(opts, WithExtractCode()) }
	if *extractURL    { opts = append(opts, WithExtractURL()) }

	if err := run(os.Stdin, os.Stdout, opts, *jsonField, *extractCode || *extractURL); err != nil {
		if errors.Is(err, errNoMatch) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "clipkit: %v\n", err)
		os.Exit(2)
	}
}
