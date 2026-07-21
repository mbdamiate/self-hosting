package execrunner

import (
	"context"
	"strings"
)

// Call records one invocation made through a Fake Runner.
type Call struct {
	Name  string
	Args  []string
	Stdin []byte
}

// Response is the canned result a Fake Runner returns for a given command.
type Response struct {
	Stdout []byte
	Err    error
}

// Fake is a Runner for unit tests: it records every call and returns a
// canned Response keyed by the space-joined command line, so subcommand
// logic can be exercised without a KVM host.
type Fake struct {
	Calls     []Call
	Responses map[string]Response
	// Sequences, if set for a key, is consumed one Response per call to
	// that key (in order); once exhausted, lookups fall back to
	// Responses. Use this when the same command is called more than once
	// in a single Run and must return different output each time (e.g.
	// `domblklist` before vs. after a snapshot is created).
	Sequences map[string][]Response
}

func NewFake() *Fake {
	return &Fake{Responses: map[string]Response{}, Sequences: map[string][]Response{}}
}

// Key builds the lookup key Fake uses for a given command invocation.
func Key(name string, args ...string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

func (f *Fake) respond(key string) (Response, bool) {
	if seq, ok := f.Sequences[key]; ok && len(seq) > 0 {
		f.Sequences[key] = seq[1:]
		return seq[0], true
	}
	resp, ok := f.Responses[key]
	return resp, ok
}

func (f *Fake) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.Calls = append(f.Calls, Call{Name: name, Args: args})
	if resp, ok := f.respond(Key(name, args...)); ok {
		return resp.Stdout, resp.Err
	}
	return nil, nil
}

func (f *Fake) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	f.Calls = append(f.Calls, Call{Name: name, Args: args, Stdin: stdin})
	if resp, ok := f.respond(Key(name, args...)); ok {
		return resp.Stdout, resp.Err
	}
	return nil, nil
}
