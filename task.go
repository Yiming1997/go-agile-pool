package agilepool

type Task interface {
	Process()
}

type TaskFunc func()

func (tf TaskFunc) Process() {
	tf()
}
