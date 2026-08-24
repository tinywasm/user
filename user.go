// Package user defines the stable identity value shared by TinyWasm libraries.
package user

import "github.com/tinywasm/model"

// SubjectID identifies a person independently of authentication and policy.
type SubjectID string

// Subject is the safe display identity returned by an authentication service.
// Roles and permissions deliberately do not belong here; rbac owns them.
type Subject struct {
	ID     SubjectID
	Email  string
	Name   string
	Avatar string
}

func (s Subject) IsNil() bool { return false }

func (s Subject) EncodeFields(w model.FieldWriter) {
	w.String("id", string(s.ID))
	w.String("email", s.Email)
	w.String("name", s.Name)
	w.String("avatar", s.Avatar)
}

func (s *Subject) DecodeFields(r model.FieldReader) {
	v, _ := r.String("id")
	s.ID = SubjectID(v)
	s.Email, _ = r.String("email")
	s.Name, _ = r.String("name")
	s.Avatar, _ = r.String("avatar")
}
