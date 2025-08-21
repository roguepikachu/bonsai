package pkg

type Response struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

func NewResponse(code int, data interface{}, message string) Response {
	return Response{
		Code:    code,
		Data:    data,
		Message: message,
	}
}