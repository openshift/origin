package osinserver

type UserStorage struct {
}

func NewUserStorage() *UserStorage {
	return &UserStorage{}
}

func (s *UserStorage) ConvertToAuthorizeToken(interface{}, *api.AuthorizeToken) error {

}

func (s *UserStorage) ConvertToAccessToken(interface{}, *api.AccessToken) error {

}

func (s *UserStorage) ConvertFromAuthorizeToken(*api.AuthorizeToken) (interface{}, error) {

}

func (s *UserStorage) ConvertFromAccessToken(*api.AccessToken) (interface{}, error) {

}
