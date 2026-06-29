package main

import "testing"

func TestParseCLIOptionsUnsafePublicWebRequiresWeb(t *testing.T) {
	if _, _, err := parseCLIOptions([]string{"--unsafe-public-web"}); err == nil {
		t.Fatal("--unsafe-public-web without --web should fail")
	}
	opts, _, err := parseCLIOptions([]string{"--web", "--addr", "0.0.0.0:8787", "--unsafe-public-web"})
	if err != nil {
		t.Fatalf("web public opt-in should parse: %v", err)
	}
	if !opts.Web || !opts.UnsafePublicWeb || opts.Addr != "0.0.0.0:8787" {
		t.Fatalf("unexpected opts: %+v", opts)
	}
}
