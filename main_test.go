package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func runWith(t *testing.T, input string, opts ...Option) string {
	t.Helper()
	var out strings.Builder
	if err := run(strings.NewReader(input), &out, opts, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return strings.TrimRight(out.String(), "\n")
}

func runPipe(t *testing.T, inputJSON string, cfg mxctlConfig) (string, error) {
	t.Helper()
	var out strings.Builder
	err := run(strings.NewReader(inputJSON), &out, nil, &cfg)
	return strings.TrimRight(out.String(), "\n"), err
}

// ---- plain text mode -------------------------------------------------------

func TestRunPlainPassthrough(t *testing.T) {
	got := runWith(t, "  hello world  ")
	if got != "hello world" {
		t.Errorf("got %q", got)
	}
}

func TestRunPlainEmpty(t *testing.T) {
	got := runWith(t, "")
	if got != "" {
		t.Errorf("got %q", got)
	}
}

func TestRunPlainMultiLine(t *testing.T) {
	got := runWith(t, "  foo  \n  bar  ", WithCollapseSpaces())
	want := "foo\nbar"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---- pipe mode -------------------------------------------------------------

func TestRunPipeModePassthrough(t *testing.T) {
	got, err := runPipe(t, `{"body":"  hello world  "}`, mxctlConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]string
	if e := json.Unmarshal([]byte(got), &obj); e != nil {
		t.Fatalf("output is not valid JSON: %v", e)
	}
	if obj["body"] != "hello world" {
		t.Errorf("body = %q, want %q", obj["body"], "hello world")
	}
}

func TestRunPipeModeExtractCode(t *testing.T) {
	got, err := runPipe(t, `{"body":"Your OTP is 482910"}`, mxctlConfig{ExtractCode: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]string
	if e := json.Unmarshal([]byte(got), &obj); e != nil {
		t.Fatalf("output is not valid JSON: %v", e)
	}
	if obj["body"] != "482910" {
		t.Errorf("body = %q, want %q", obj["body"], "482910")
	}
}

func TestRunPipeModeNoMatchReturnsErrNoMatch(t *testing.T) {
	_, err := runPipe(t, `{"body":"no code here"}`, mxctlConfig{ExtractCode: true})
	if !errors.Is(err, errNoMatch) {
		t.Errorf("expected errNoMatch, got %v", err)
	}
}

func TestRunPipeModePreservesFields(t *testing.T) {
	input := `{"body":"OTP: 111222","sender":"@alice:example.com","room_name":"Work"}`
	got, err := runPipe(t, input, mxctlConfig{ExtractCode: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]string
	if e := json.Unmarshal([]byte(got), &obj); e != nil {
		t.Fatalf("output is not valid JSON: %v", e)
	}
	if obj["body"] != "111222" {
		t.Errorf("body = %q, want %q", obj["body"], "111222")
	}
	if obj["sender"] != "@alice:example.com" {
		t.Errorf("sender = %q, want preserved", obj["sender"])
	}
	if obj["room_name"] != "Work" {
		t.Errorf("room_name = %q, want preserved", obj["room_name"])
	}
}

func TestRunPipeModePreservesCustomFields(t *testing.T) {
	input := `{"body":"hello","urgency":"critical","custom_flag":true}`
	got, err := runPipe(t, input, mxctlConfig{Collapse: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]json.RawMessage
	if e := json.Unmarshal([]byte(got), &obj); e != nil {
		t.Fatalf("output is not valid JSON: %v", e)
	}
	if string(obj["urgency"]) != `"critical"` {
		t.Errorf("urgency = %s, want preserved", obj["urgency"])
	}
	if string(obj["custom_flag"]) != `true` {
		t.Errorf("custom_flag = %s, want preserved", obj["custom_flag"])
	}
}

func TestRunPipeModeExtractURL(t *testing.T) {
	got, err := runPipe(t, `{"body":"check https://example.com now"}`, mxctlConfig{ExtractURL: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]string
	if e := json.Unmarshal([]byte(got), &obj); e != nil {
		t.Fatalf("output is not valid JSON: %v", e)
	}
	if obj["body"] != "https://example.com" {
		t.Errorf("body = %q, want %q", obj["body"], "https://example.com")
	}
}

func TestRunPipeModeNoURLReturnsErrNoMatch(t *testing.T) {
	_, err := runPipe(t, `{"body":"no url here"}`, mxctlConfig{ExtractURL: true})
	if !errors.Is(err, errNoMatch) {
		t.Errorf("expected errNoMatch, got %v", err)
	}
}

func TestRunPipeModeCollapse(t *testing.T) {
	got, err := runPipe(t, `{"body":"  foo   bar  "}`, mxctlConfig{Collapse: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]string
	json.Unmarshal([]byte(got), &obj)
	if obj["body"] != "foo bar" {
		t.Errorf("body = %q, want %q", obj["body"], "foo bar")
	}
}

func TestRunPipeModeMaxLen(t *testing.T) {
	got, err := runPipe(t, `{"body":"hello world"}`, mxctlConfig{MaxLen: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]string
	json.Unmarshal([]byte(got), &obj)
	if obj["body"] != "hello…" {
		t.Errorf("body = %q, want %q", obj["body"], "hello…")
	}
}

func TestRunPipeModeEmptyBodyNoMatchOnExtract(t *testing.T) {
	_, err := runPipe(t, `{"sender":"@alice:example.com"}`, mxctlConfig{ExtractCode: true})
	if !errors.Is(err, errNoMatch) {
		t.Errorf("expected errNoMatch for missing body, got %v", err)
	}
}

// ---- flag validation -------------------------------------------------------

