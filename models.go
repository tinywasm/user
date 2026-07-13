package user

import (
	"github.com/tinywasm/form/input"
	"github.com/tinywasm/model"
)

var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "id", Type: model.FieldText, DB: &model.FieldDB{PK: true}},
		{Name: "email", Type: model.FieldText, DB: &model.FieldDB{Unique: true}},
		{Name: "name", Type: model.FieldText},
		{Name: "phone", Type: model.FieldText},
		{Name: "status", Type: model.FieldText},
		{Name: "created_at", Type: model.FieldInt},
		{Name: "roles", Type: model.FieldStructSlice, Ref: &RoleModel, Exclude: true},
		{Name: "permissions", Type: model.FieldStructSlice, Ref: &PermissionModel, Exclude: true},
	},
}

var SessionModel = model.Definition{
	Name: "session",
	Fields: model.Fields{
		{Name: "id", Type: model.FieldText, DB: &model.FieldDB{PK: true}},
		{Name: "user_id", Type: model.FieldText, DB: &model.FieldDB{RefColumn: "id"}, Ref: &UserModel},
		{Name: "expires_at", Type: model.FieldInt},
		{Name: "ip", Type: model.FieldText},
		{Name: "user_agent", Type: model.FieldText},
		{Name: "created_at", Type: model.FieldInt},
	},
}

var IdentityModel = model.Definition{
	Name: "identity",
	Fields: model.Fields{
		{Name: "id", Type: model.FieldText, DB: &model.FieldDB{PK: true}},
		{Name: "user_id", Type: model.FieldText, DB: &model.FieldDB{RefColumn: "id"}, Ref: &UserModel},
		{Name: "provider", Type: model.FieldText},
		{Name: "provider_id", Type: model.FieldText},
		{Name: "email", Type: model.FieldText},
		{Name: "created_at", Type: model.FieldInt},
	},
}

var RoleModel = model.Definition{
	Name: "role",
	Fields: model.Fields{
		{Name: "id", Type: model.FieldText, DB: &model.FieldDB{PK: true}},
		{Name: "code", Type: model.FieldText},
		{Name: "name", Type: model.FieldText},
		{Name: "description", Type: model.FieldText},
	},
}

var UserRoleModel = model.Definition{
	Name: "user_role",
	Fields: model.Fields{
		{Name: "user_id", Type: model.FieldText, DB: &model.FieldDB{PK: true, RefColumn: "id"}, Ref: &UserModel},
		{Name: "role_id", Type: model.FieldText, DB: &model.FieldDB{PK: true, RefColumn: "id"}, Ref: &RoleModel},
	},
}

var PermissionModel = model.Definition{
	Name: "permission",
	Fields: model.Fields{
		{Name: "id", Type: model.FieldText, DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.FieldText},
		{Name: "resource", Type: model.FieldText},
		{Name: "action", Type: model.FieldText},
	},
}

var RolePermissionModel = model.Definition{
	Name: "role_permission",
	Fields: model.Fields{
		{Name: "role_id", Type: model.FieldText, DB: &model.FieldDB{PK: true, RefColumn: "id"}, Ref: &RoleModel},
		{Name: "permission_id", Type: model.FieldText, DB: &model.FieldDB{PK: true, RefColumn: "id"}, Ref: &PermissionModel},
	},
}

var LANIPModel = model.Definition{
	Name: "lanip",
	Fields: model.Fields{
		{Name: "id", Type: model.FieldText, DB: &model.FieldDB{PK: true}},
		{Name: "user_id", Type: model.FieldText, DB: &model.FieldDB{RefColumn: "id"}, Ref: &UserModel},
		{Name: "ip", Type: model.FieldText},
		{Name: "label", Type: model.FieldText},
		{Name: "created_at", Type: model.FieldInt},
	},
}

var OAuthStateModel = model.Definition{
	Name: "oauth_state",
	Fields: model.Fields{
		{Name: "state", Type: model.FieldText, DB: &model.FieldDB{PK: true}},
		{Name: "provider", Type: model.FieldText},
		{Name: "expires_at", Type: model.FieldInt},
		{Name: "created_at", Type: model.FieldInt},
	},
}

var LoginDataModel = model.Definition{
	Name: "login_data",
	Fields: model.Fields{
		{Name: "email", Type: model.FieldText, NotNull: true, Widget: input.Email()},
		{Name: "password", Type: model.FieldText, NotNull: true, Widget: input.Password()},
	},
}

var RegisterDataModel = model.Definition{
	Name: "register_data",
	Fields: model.Fields{
		{Name: "name", Type: model.FieldText, NotNull: true, Widget: input.Text()},
		{Name: "email", Type: model.FieldText, NotNull: true, Widget: input.Email()},
		{Name: "password", Type: model.FieldText, NotNull: true, Widget: input.Password()},
		{Name: "phone", Type: model.FieldText, Widget: input.Phone()},
	},
}

var ProfileDataModel = model.Definition{
	Name: "profile_data",
	Fields: model.Fields{
		{Name: "name", Type: model.FieldText, NotNull: true, Widget: input.Text()},
		{Name: "phone", Type: model.FieldText, Widget: input.Phone()},
	},
}

var PasswordDataModel = model.Definition{
	Name: "password_data",
	Fields: model.Fields{
		{Name: "current", Type: model.FieldText, NotNull: true, Widget: input.Password()},
		{Name: "new", Type: model.FieldText, NotNull: true, Widget: input.Password()},
		{Name: "confirm", Type: model.FieldText, NotNull: true, Widget: input.Password()},
	},
}
