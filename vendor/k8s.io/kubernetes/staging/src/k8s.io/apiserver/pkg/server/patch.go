package server

func (s *GenericAPIServer) RemoveOpenAPIData() {
	s.openAPIConfig = nil
}
