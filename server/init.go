package userserver

import (
	"github.com/tinywasm/form"
	"github.com/tinywasm/form/input"
)

func init() {
	form.RegisterInput(
		input.Text(),
		input.Password(),
	)
}
