package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// mxctlConfig holds options received via --config flag.
type mxctlConfig struct {
	Collapse        bool   `json:"collapse"`
	Cutset          string `json:"cutset"`
	MaxLen          int    `json:"max_len"`
	StripTimestamps bool   `json:"strip_timestamps"`
	StripBorders    bool   `json:"strip_borders"`
	ExtractCode     bool   `json:"extract_code"`
	ExtractURL      bool   `json:"extract_url"`
}

// errNoMatch is returned by runPipeMode when extraction finds nothing.
// The caller exits with code 1 to signal mxctl to abort the pipe chain.
var errNoMatch = errors.New("no match")

func optsFromConfig(cfg mxctlConfig) []Option {
	var opts []Option
	if cfg.Collapse        { opts = append(opts, WithCollapseSpaces()) }
	if cfg.Cutset != ""    { opts = append(opts, WithCutset(cfg.Cutset)) }
	if cfg.MaxLen > 0      { opts = append(opts, WithMaxLen(cfg.MaxLen)) }
	if cfg.StripTimestamps { opts = append(opts, WithStripTimestamps()) }
	if cfg.StripBorders    { opts = append(opts, WithStripBorders()) }
	if cfg.ExtractCode     { opts = append(opts, WithExtractCode()) }
	if cfg.ExtractURL      { opts = append(opts, WithExtractURL()) }
	return opts
}

func runPipeMode(r io.Reader, w io.Writer, cfg mxctlConfig) error {
	data, err := io.ReadAll(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: read error: %v\n", err)
		os.Exit(1)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: json parse: %v\n", err)
		os.Exit(1)
	}

	var body string
	if raw, ok := obj["body"]; ok {
		if err := json.Unmarshal(raw, &body); err != nil {
			fmt.Fprintf(os.Stderr, "clipkit: body parse: %v\n", err)
			os.Exit(1)
		}
	}

	out, err := Parse(body, optsFromConfig(cfg)...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: %v\n", err)
		os.Exit(1)
	}
	if out == "" {
		return errNoMatch
	}

	outJSON, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: json marshal: %v\n", err)
		os.Exit(1)
	}
	obj["body"] = outJSON

	result, err := json.Marshal(obj)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: json marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(w, string(result))
	return nil
}

func run(r io.Reader, w io.Writer, opts []Option, cfg *mxctlConfig) error {
	if cfg != nil {
		return runPipeMode(r, w, *cfg)
	}

	scanner := bufio.NewScanner(r)
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for scanner.Scan() {
		out, err := Parse(scanner.Text(), opts...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "clipkit: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(bw, out)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "clipkit: read error: %v\n", err)
		os.Exit(1)
	}
	return nil
}

func main() {
	configJSON   := flag.String("config", "", "JSON config object (mxctl pipe mode); must be paired with --event")
	eventJSON    := flag.String("event", "", "Original Matrix event JSON (mxctl pipe mode); must be paired with --config")
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
		fmt.Fprintf(os.Stderr, "Pipe mode (mxctl integration):\n")
		fmt.Fprintf(os.Stderr, "  Activated by --config. Reads a JSON object from stdin, transforms the\n")
		fmt.Fprintf(os.Stderr, "  'body' field, and writes the updated JSON object to stdout.\n")
		fmt.Fprintf(os.Stderr, "  Exits 2 if extraction finds nothing (silently stops the mxctl pipe chain).\n")
		fmt.Fprintf(os.Stderr, "  --config and --event must always be used together; stdin must be JSON.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  echo '  hello   world  ' | clipkit --collapse\n")
		fmt.Fprintf(os.Stderr, "  cat app.log | clipkit --strip-borders --collapse --strip-timestamps\n")
		fmt.Fprintf(os.Stderr, "  echo '{\"body\":\"OTP: 482910\"}' | clipkit --config '{\"extract_code\":true}'\n")
	}

	flag.Parse()

	// --config and --event must come together
	configSet := *configJSON != ""
	eventSet  := *eventJSON != ""
	if configSet != eventSet {
		fmt.Fprintf(os.Stderr, "clipkit: --config and --event must be used together\n")
		os.Exit(2)
	}

	var cfgPtr *mxctlConfig
	if configSet {
		var cfg mxctlConfig
		if err := json.Unmarshal([]byte(*configJSON), &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "clipkit: --config parse: %v\n", err)
			os.Exit(2)
		}
		var otherFlagsSet bool
		flag.Visit(func(f *flag.Flag) {
			if f.Name != "config" && f.Name != "event" {
				otherFlagsSet = true
			}
		})
		if otherFlagsSet {
			fmt.Fprintf(os.Stderr, "clipkit: --config and --event are mutually exclusive with all other flags\n")
			os.Exit(2)
		}
		cfgPtr = &cfg
	}

	var opts []Option
	if *collapse      { opts = append(opts, WithCollapseSpaces()) }
	if *cutset != ""  { opts = append(opts, WithCutset(*cutset)) }
	if *maxLen > 0    { opts = append(opts, WithMaxLen(*maxLen)) }
	if *stripTs       { opts = append(opts, WithStripTimestamps()) }
	if *stripBorders  { opts = append(opts, WithStripBorders()) }
	if *extractCode   { opts = append(opts, WithExtractCode()) }
	if *extractURL    { opts = append(opts, WithExtractURL()) }

	if err := run(os.Stdin, os.Stdout, opts, cfgPtr); err != nil {
		if errors.Is(err, errNoMatch) {
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "clipkit: %v\n", err)
		os.Exit(2)
	}
}
