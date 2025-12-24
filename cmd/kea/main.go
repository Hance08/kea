package main

import (
	"github.com/hance08/kea/cmd"
	"github.com/hance08/kea/migrations"
)

func main() {
	cmd.Execute(migrations.FS)
}
