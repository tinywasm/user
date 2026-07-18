package user

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

// NewView builds the user-administration Presenter — the tech-agnostic engine a
// renderer (crudview, or any other) wraps. The app decides which renderer draws it.
func NewView(caller router.Caller) view.Presenter {
	var byID []*User
	record := &User{}

	return view.New(
		caller,
		record,
		OpListUsers,
		func() model.FielderSlice { return &UserList{} },
		func(list model.FielderSlice) []view.Item {
			l := list.(*UserList)
			items := make([]view.Item, l.Len())
			byID = make([]*User, l.Len())
			for i := 0; i < l.Len(); i++ {
				it := l.At(i).(*User)
				byID[i] = it
				items[i] = view.Item{ID: it.Id, Label: it.Name, Description: it.Email}
			}
			return items
		},
		view.WithTitle("Usuarios"),
		view.WithSaveOp(OpUpsertUser),
		view.WithDeleteOp(OpDeleteUser),
		view.WithFill(func(id string) model.Model {
			if id == "" {
				return nil
			}
			for _, it := range byID {
				if it != nil && it.Id == id {
					return it
				}
			}
			return nil
		}),
	)
}
