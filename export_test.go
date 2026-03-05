// export_test.go — expone funciones internas para tests
package user

var (
	GenerateJWT = generateJWT
	ValidateJWT = validateJWT
)
