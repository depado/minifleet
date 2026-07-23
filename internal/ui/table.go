package ui

import (
	"github.com/depado/gorich/table"
	"github.com/depado/gorich/table/box"
)

func NewTable(headers ...string) *table.Table {
	return table.NewTableWithOptions(headers,
		table.WithBox(box.ROUNDED),
		table.WithExpand(),
	)
}

func NewTitledTable(title string, headers ...string) *table.Table {
	return table.NewTableWithOptions(headers,
		table.WithBox(box.ROUNDED),
		table.WithExpand(),
		table.WithTitle(title),
	)
}
