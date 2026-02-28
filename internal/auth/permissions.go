package auth

func init() {
	RegisterPermission(Permission{Key: "user:create", Name: "Create Users", Description: "Create new user accounts within the tenant"})
	RegisterPermission(Permission{Key: "user:read", Name: "View Users", Description: "View user profiles"})
	RegisterPermission(Permission{Key: "user:update", Name: "Update Users", Description: "Update user details"})
	RegisterPermission(Permission{Key: "user:delete", Name: "Delete Users", Description: "Delete user accounts"})
	RegisterPermission(Permission{Key: "role:read", Name: "View Roles", Description: "View roles and their permissions"})
	RegisterPermission(Permission{Key: "role:manage", Name: "Manage Roles", Description: "Create roles and assign permissions"})
}
