package main

import (
	"encoding/json"
	"fmt"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		os.Exit(1)
	}
	p, e := manifest.Load(os.Args[1])
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	digest, e := p.ContentDigest()
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"manifestDigest": p.Digest, "packageDigest": digest})
}
