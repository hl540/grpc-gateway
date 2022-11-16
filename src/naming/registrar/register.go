package registrar

type Registrar interface {
	// Register 注册服务
	Register() error
	// UnRegister 注销服务
	UnRegister() error
}
