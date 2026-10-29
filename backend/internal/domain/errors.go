package domain

type Error struct {
	Status       int
	Code, Detail string
}

func (e *Error) Error() string           { return e.Detail }
func Invalid(detail string) error        { return &Error{400, "VALIDATION_ERROR", detail} }
func Conflict(code, detail string) error { return &Error{409, code, detail} }
func Missing() error                     { return &Error{404, "NOT_FOUND", "Recurso não encontrado."} }
func Unauthorized() error                { return &Error{401, "UNAUTHORIZED", "Credencial inválida ou expirada."} }
