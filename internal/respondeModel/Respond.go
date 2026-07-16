package RespModel

// 响应部分使用了泛型,保存各种信息
type Respond[T any] struct {
	Code int
	Msg  string
	Data T
}
