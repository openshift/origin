package ldapserver

// SearchRequest is a definition of the Search Operation
// baseObject - The name of the base object entry (or possibly the root) relative to which the Search is to be performed
type SearchRequest struct {
	BaseObject   []byte
	Scope        int
	DerefAliases int
	SizeLimit    int
	TimeLimit    int
	TypesOnly    bool
	Attributes   [][]byte
	Filter       string
}

func (s *SearchRequest) GetTypesOnly() bool {
	return s.TypesOnly
}

func (s *SearchRequest) GetAttributes() [][]byte {
	return s.Attributes
}
func (s *SearchRequest) GetFilter() string {
	return s.Filter
}
func (s *SearchRequest) GetBaseObject() []byte {
	return s.BaseObject
}
func (s *SearchRequest) GetScope() int {
	return s.Scope
}
func (s *SearchRequest) GetDerefAliases() int {
	return s.DerefAliases
}
func (s *SearchRequest) GetSizeLimit() int {
	return s.SizeLimit
}
func (s *SearchRequest) GetTimeLimit() int {
	return s.TimeLimit
}

// SearchResultEntry represents an entry found during the Search
type SearchResultEntry struct {
	MessageID  int
	dN         string
	attributes PartialAttributeList
}

func (e *SearchResultEntry) SetMessageID(ID int) {
	e.MessageID = ID
}

func (e *SearchResultEntry) SetDn(dn string) {
	e.dN = dn
}

func (e *SearchResultEntry) AddAttribute(name AttributeDescription, values ...AttributeValue) {
	var ea = PartialAttribute{type_: name, vals: values}
	e.attributes.add(ea)
}

type SearchResponse struct {
	ldapResult
	referrals []string
	//Controls []Control
}

func NewSearchResultDoneResponse(resultCode int) *SearchResponse {
	r := &SearchResponse{}
	r.ResultCode = resultCode
	return r
}

func NewSearchResultEntry() *SearchResultEntry {
	r := &SearchResultEntry{}
	return r
}
