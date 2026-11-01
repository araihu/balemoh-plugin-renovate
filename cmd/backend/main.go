package main

import (
	"context"
	"fmt"
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/runtime"
	"os"
)

func main() {
	p, e := manifest.Load(".")
	if e != nil {
		fmt.Fprintln(os.Stderr, "invalid plugin package")
		os.Exit(1)
	}
	runtime.ServeBackend(p, advisory.New(context.Background()))
}
