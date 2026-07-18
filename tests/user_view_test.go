package tests

import (
	"testing"

	"github.com/tinywasm/model"
	"github.com/tinywasm/user"
)

type fakeCaller struct {
	lastOp   string
	lastArgs model.Encodable
}

func (c *fakeCaller) Call(op string, args model.Encodable, out model.Decodable, cb func(error)) {
	c.lastOp = op
	c.lastArgs = args
}

func (c *fakeCaller) Dispatch(s string, e model.Encodable) {}

func TestNewView(t *testing.T) {
	fc := &fakeCaller{}
	v := user.NewView(fc)
	if v == nil {
		t.Fatal("expected view to not be nil")
	}
	if v.Title() != "Usuarios" {
		t.Errorf("expected title 'Usuarios', got %s", v.Title())
	}
}
