package main

import (
	"fmt"
	"github.com/araihu/balemoh-plugin-renovate/internal/pluginui"
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
	runtime.Serve(p, pluginui.Handler{Manifest: p.Manifest})
}
