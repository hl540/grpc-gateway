package register

type Registrar interface {
	Register() error
	UnRegister() error
}
