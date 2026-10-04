package web

import (
	"embed"
	"io/fs"
)

// Templates are embedded independently and never exposed through /static/.
//
//go:embed templates/*
var TemplatesFS embed.FS

//go:embed static/css/* static/js/* static/logo.png
var StaticFS embed.FS

//go:embed plugins/yub-wpanel-optimizer/*
var pluginAssets embed.FS

// PluginFS retains installed plugin paths while the source lives under web/.
var PluginFS = pluginSubFS()

func pluginSubFS() fs.FS {
	assets, err := fs.Sub(pluginAssets, "plugins")
	if err != nil {
		panic(err)
	}
	return assets
}
