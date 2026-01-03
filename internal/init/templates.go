package initcmd

import "embed"

//go:embed templates/*
var templateFS embed.FS

// MustReadTemplate reads a template file from the embedded filesystem.
// Panics if the file cannot be read.
func MustReadTemplate(name string) string {
	data, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		panic("failed to read embedded template: " + err.Error())
	}
	return string(data)
}
