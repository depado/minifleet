package ui

import (
	"github.com/depado/gorich"
	"github.com/depado/gorich/console"
	"github.com/depado/gorich/table"
	"github.com/depado/gorich/table/box"
)

var DefaultConsole = console.New()

func NewTable(headers ...string) *table.Table {
	return table.NewTableWithOptions(headers,
		table.WithBox(box.ROUNDED),
		table.WithExpand(),
	)
}

func PrintError(msg string) { gorich.Print("[red]✗[/] " + msg) }
func PrintInfo(msg string)  { gorich.Print("[cyan]→[/] " + msg) }
func PrintDim(msg string)   { gorich.Print("[dim]" + msg + "[/]") }

// DefaultPrint renders a message with rich-text tag support, no prefix.
func DefaultPrint(msg string) { gorich.Print(msg) }
