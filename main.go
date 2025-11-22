/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"embed"

	"github.com/hance08/kea/cmd"
)

//go:embed migrations
var migrationsFS embed.FS

func main() {
	cmd.Execute(migrationsFS)
}
