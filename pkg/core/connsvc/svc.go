package connsvc

type ConnManager interface {
}

type Service struct {
	connmng ConnManager
}

func New(connmng ConnManager) *Service {
	return &Service{
		connmng: connmng,
	}
}
