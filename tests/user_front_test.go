//go:build wasm

package tests

import (
	"testing"

	"github.com/tinywasm/model"
	"github.com/tinywasm/user"
)

type fakeCallerWasm struct {
	lastOp   string
	lastArgs model.Encodable
}

func (c *fakeCallerWasm) Call(op string, args model.Encodable, out model.Decodable, cb func(error)) {
	c.lastOp = op
	c.lastArgs = args
	if op == "list_users" {
		if l, ok := out.(*user.UserList); ok {
			u1 := l.Append().(*user.User)
			u1.Id = "u1"
			u1.Name = "User One"
			u1.Email = "u1@test.com"

			u2 := l.Append().(*user.User)
			u2.Id = "u2"
			u2.Name = "User Two"
			u2.Email = "u2@test.com"
		}
		if cb != nil {
			cb(nil)
		}
	} else if op == "upsert_user" || op == "delete_user" {
		if cb != nil {
			cb(nil)
		}
	}
}

func (c *fakeCallerWasm) Dispatch(s string, e model.Encodable) {}

func TestNewView(t *testing.T) {
	fc := &fakeCallerWasm{}
	v := user.NewView(fc)
	if v == nil {
		t.Fatal("expected view to not be nil")
	}

	// 1. Verify Title
	if v.Title() != "Usuarios" {
		t.Errorf("expected title 'Usuarios', got %s", v.Title())
	}

	// 2. Reload to load items
	v.Reload()

	// 3. Verify items projection
	items := v.Items()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "u1" || items[0].Label != "User One" || items[0].Description != "u1@test.com" {
		t.Errorf("wrong item projection at 0: %+v", items[0])
	}
	if items[1].ID != "u2" || items[1].Label != "User Two" || items[1].Description != "u2@test.com" {
		t.Errorf("wrong item projection at 1: %+v", items[1])
	}

	// 4. Select Item and verify Fill callback logic via return value of Select()
	rec := v.Select("u1")
	if rec == nil {
		t.Fatal("expected selected record to not be nil")
	}
	u, ok := rec.(*user.User)
	if !ok {
		t.Fatalf("expected record of type *user.User, got %T", rec)
	}
	if u.Id != "u1" || u.Name != "User One" {
		t.Errorf("wrong record selected: %+v", u)
	}

	// 5. Save record and verify caller payload
	u.Name = "Updated Via Presenter"
	v.Save(u)
	if fc.lastOp != "upsert_user" {
		t.Errorf("expected upsert_user op on save, got %s", fc.lastOp)
	}
	savedUser, ok := fc.lastArgs.(*user.User)
	if !ok || savedUser.Name != "Updated Via Presenter" {
		t.Errorf("wrong payload saved: %+v", fc.lastArgs)
	}

	// 6. Delete record and verify caller payload
	v.Delete("u2")
	if fc.lastOp != "delete_user" {
		t.Errorf("expected delete_user op on delete, got %s", fc.lastOp)
	}
	deletedUser, ok := fc.lastArgs.(*user.User)
	if !ok || deletedUser.Id != "u2" {
		t.Errorf("wrong payload deleted: %+v", fc.lastArgs)
	}
}
