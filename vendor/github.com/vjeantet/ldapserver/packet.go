package ldapserver

import (
	"bufio"
	"errors"
	"fmt"
	"log"

	ber "github.com/vjeantet/asn1-ber"
)

type messagePacket struct {
	Packet *ber.Packet
}

func (msg *messagePacket) getOperation() int {
	return int(msg.Packet.Children[1].Tag)
}

func (msg *messagePacket) readMessage() (m Message, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid packet received hex=%x", msg.Packet.Bytes())
		}
	}()

	m.MessageID = int(msg.Packet.Children[0].Value.(uint64))
	switch msg.getOperation() {
	case ApplicationBindRequest:
		var br BindRequest
		br.Login = msg.Packet.Children[1].Children[1].Data.Bytes()
		br.Password = msg.Packet.Children[1].Children[2].Data.Bytes()
		br.Version = int(msg.Packet.Children[1].Children[0].Value.(uint64))
		m.protocolOp = br
		return m, nil

	case ApplicationUnbindRequest:
		var ur UnbindRequest
		m.protocolOp = ur
		return m, nil

	case ApplicationSearchRequest:
		var sr SearchRequest
		sr.BaseObject = msg.Packet.Children[1].Children[0].Data.Bytes()
		sr.Scope = int(msg.Packet.Children[1].Children[1].Value.(uint64))
		sr.DerefAliases = int(msg.Packet.Children[1].Children[2].Value.(uint64))
		sr.SizeLimit = int(msg.Packet.Children[1].Children[3].Value.(uint64))
		sr.TimeLimit = int(msg.Packet.Children[1].Children[4].Value.(uint64))
		sr.TypesOnly = msg.Packet.Children[1].Children[5].Value.(bool)

		var ldaperr error
		sr.Filter, ldaperr = decompileFilter(msg.Packet.Children[1].Children[6])
		if ldaperr != nil {
			log.Printf("error decompiling searchrequestfilter %s", ldaperr)
		}

		for i := range msg.Packet.Children[1].Children[7].Children {
			sr.Attributes = append(sr.Attributes, msg.Packet.Children[1].Children[7].Children[i].Data.Bytes())
		}
		m.protocolOp = sr
		return m, nil

	case ApplicationAddRequest:
		var r AddRequest
		r.entry = LDAPDN(msg.Packet.Children[1].Children[0].Data.Bytes())

		for i := range msg.Packet.Children[1].Children[1].Children {
			rattribute := Attribute{type_: AttributeDescription(msg.Packet.Children[1].Children[1].Children[i].Children[0].Data.Bytes())}
			for j := range msg.Packet.Children[1].Children[1].Children[i].Children[1].Children {
				rattribute.vals = append(rattribute.vals, AttributeValue(msg.Packet.Children[1].Children[1].Children[i].Children[1].Children[j].Data.Bytes()))
			}
			r.attributes = append(r.attributes, rattribute)
		}
		m.protocolOp = r
		return m, nil

	case ApplicationModifyRequest:
		var r ModifyRequest
		r.object = LDAPDN(msg.Packet.Children[1].Children[0].Data.Bytes())
		for i := range msg.Packet.Children[1].Children[1].Children {
			operation := int(msg.Packet.Children[1].Children[1].Children[i].Children[0].Value.(uint64))
			attributeName := msg.Packet.Children[1].Children[1].Children[i].Children[1].Children[0].Value.(string)
			modifyRequestChange := modifyRequestChange{operation: operation}
			rattribute := PartialAttribute{type_: AttributeDescription(attributeName)}
			for j := range msg.Packet.Children[1].Children[1].Children[i].Children[1].Children[1].Children {
				value := msg.Packet.Children[1].Children[1].Children[i].Children[1].Children[1].Children[j].Value.(string)
				rattribute.vals = append(rattribute.vals, AttributeValue(value))
			}
			modifyRequestChange.modification = rattribute
			r.changes = append(r.changes, modifyRequestChange)
		}
		m.protocolOp = r
		return m, nil

	case ApplicationDelRequest:
		var r DeleteRequest
		r = DeleteRequest(msg.Packet.Children[1].Data.Bytes())
		m.protocolOp = r
		return m, nil

	case ApplicationExtendedRequest:
		var r ExtendedRequest
		r.requestName = LDAPOID(msg.Packet.Children[1].Children[0].Data.Bytes())
		if len(msg.Packet.Children[1].Children) > 1 {
			r.requestValue = msg.Packet.Children[1].Children[1].Data.Bytes()
		}
		m.protocolOp = r
		return m, nil

	case ApplicationAbandonRequest:
		var r AbandonRequest
		r = AbandonRequest(msg.Packet.Children[1].Value.(uint64))
		m.protocolOp = r
		return m, nil

	case ApplicationCompareRequest:
		var r CompareRequest

		r.entry = LDAPDN(msg.Packet.Children[1].Children[0].Value.(string))
		r.ava = AttributeValueAssertion{
			attributeDesc:  AttributeDescription(msg.Packet.Children[1].Children[1].Children[0].Value.(string)),
			assertionValue: AssertionValue(msg.Packet.Children[1].Children[1].Children[1].Value.(string))}
		m.protocolOp = r
		return m, nil
	default:
		return m, fmt.Errorf("unknow ldap operation [operation=%d]", msg.getOperation())
	}

}

func decompileFilter(packet *ber.Packet) (ret string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("error decompiling filter")
		}
	}()
	ret = "("
	err = nil
	childStr := ""

	switch packet.Tag {
	case FilterAnd:
		ret += "&"
		for _, child := range packet.Children {
			childStr, err = decompileFilter(child)
			if err != nil {
				return
			}
			ret += childStr
		}
	case FilterOr:
		ret += "|"
		for _, child := range packet.Children {
			childStr, err = decompileFilter(child)
			if err != nil {
				return
			}
			ret += childStr
		}
	case FilterNot:
		ret += "!"
		childStr, err = decompileFilter(packet.Children[0])
		if err != nil {
			return
		}
		ret += childStr

	case FilterSubstrings:
		ret += ber.DecodeString(packet.Children[0].Data.Bytes())
		ret += "="
		switch packet.Children[1].Children[0].Tag {
		case FilterSubstringsInitial:
			ret += ber.DecodeString(packet.Children[1].Children[0].Data.Bytes()) + "*"
		case FilterSubstringsAny:
			ret += "*" + ber.DecodeString(packet.Children[1].Children[0].Data.Bytes()) + "*"
		case FilterSubstringsFinal:
			ret += "*" + ber.DecodeString(packet.Children[1].Children[0].Data.Bytes())
		}
	case FilterEqualityMatch:
		ret += ber.DecodeString(packet.Children[0].Data.Bytes())
		ret += "="
		ret += ber.DecodeString(packet.Children[1].Data.Bytes())
	case FilterGreaterOrEqual:
		ret += ber.DecodeString(packet.Children[0].Data.Bytes())
		ret += ">="
		ret += ber.DecodeString(packet.Children[1].Data.Bytes())
	case FilterLessOrEqual:
		ret += ber.DecodeString(packet.Children[0].Data.Bytes())
		ret += "<="
		ret += ber.DecodeString(packet.Children[1].Data.Bytes())
	case FilterPresent:
		if 0 == len(packet.Children) {
			ret += ber.DecodeString(packet.Data.Bytes())
		} else {
			ret += ber.DecodeString(packet.Children[0].Data.Bytes())
		}
		ret += "=*"
	case FilterApproxMatch:
		ret += ber.DecodeString(packet.Children[0].Data.Bytes())
		ret += "~="
		ret += ber.DecodeString(packet.Children[1].Data.Bytes())
	}

	ret += ")"
	return
}

func readMessagePacket(br *bufio.Reader) (*messagePacket, error) {
	p, err := ber.ReadPacket(br)
	//ber.PrintPacket(p)
	messagePacket := &messagePacket{Packet: p}
	return messagePacket, err
}

func newMessagePacket(lr response) *ber.Packet {
	switch v := lr.(type) {
	case *BindResponse:
		var b = lr.(*BindResponse)
		packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
		packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagInteger, uint64(b.MessageID), "MessageID"))
		bindResponse := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationBindResponse, nil, "Bind Response")
		bindResponse.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagEnumerated, uint64(b.ResultCode), "ResultCode"))
		bindResponse.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, string(b.MatchedDN), "MatchedDN"))
		bindResponse.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, b.DiagnosticMessage, "DiagnosticMessage"))
		packet.AppendChild(bindResponse)
		return packet

	case *SearchResponse:
		var res = lr.(*SearchResponse)
		packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
		packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagInteger, uint64(res.MessageID), "MessageID"))
		searchResultDone := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationSearchResultDone, nil, "Search done")
		searchResultDone.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagEnumerated, uint64(res.ResultCode), "ResultCode"))
		searchResultDone.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, string(res.MatchedDN), "MatchedDN"))
		searchResultDone.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, res.DiagnosticMessage, "DiagnosticMessage"))
		packet.AppendChild(searchResultDone)
		return packet

	case *SearchResultEntry:
		var s = lr.(*SearchResultEntry)
		packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
		packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagInteger, uint64(s.MessageID), "MessageID"))
		searchResponse := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationSearchResultEntry, nil, "SearchResultEntry")
		searchResponse.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, s.dN, "LDAPDN"))
		attributesList := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "Attributes List")
		for j := range s.attributes {
			attributes := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "attributes")
			attributes.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, string(s.attributes[j].GetDescription()), "type"))
			values := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSet, nil, "values")
			for k := range s.attributes[j].vals {
				values.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, string(s.attributes[j].vals[k]), "val"))
			}
			attributes.AppendChild(values)
			attributesList.AppendChild(attributes)

		}
		searchResponse.AppendChild(attributesList)
		packet.AppendChild(searchResponse)
		return packet

	case *ExtendedResponse:
		var b = lr.(*ExtendedResponse)
		packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
		packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagInteger, uint64(b.MessageID), "MessageID"))
		extendedResponse := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationExtendedResponse, nil, "Extended Response")
		extendedResponse.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagEnumerated, uint64(b.ResultCode), "ResultCode"))
		extendedResponse.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, string(b.MatchedDN), "MatchedDN"))
		extendedResponse.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, b.DiagnosticMessage, "DiagnosticMessage"))
		extendedResponse.AppendChild(ber.NewString(ber.ClassContext, ber.TypePrimative, 10, string(b.ResponseName), "responsename"))
		extendedResponse.AppendChild(ber.NewString(ber.ClassContext, ber.TypePrimative, 11, b.ResponseValue, "responsevalue"))
		packet.AppendChild(extendedResponse)
		return packet

	case *AddResponse:
		var res = lr.(*AddResponse)
		packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
		packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagInteger, uint64(res.MessageID), "MessageID"))
		packet2 := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationAddResponse, nil, "Add response")
		packet2.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagEnumerated, uint64(res.ResultCode), "ResultCode"))
		packet2.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, string(res.MatchedDN), "MatchedDN"))
		packet2.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, res.DiagnosticMessage, "DiagnosticMessage"))
		packet.AppendChild(packet2)
		return packet

	case *DeleteResponse:
		var res = lr.(*DeleteResponse)
		packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
		packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagInteger, uint64(res.MessageID), "MessageID"))
		packet2 := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationDelResponse, nil, "Delete response")
		packet2.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagEnumerated, uint64(res.ResultCode), "ResultCode"))
		packet2.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, string(res.MatchedDN), "MatchedDN"))
		packet2.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, res.DiagnosticMessage, "DiagnosticMessage"))
		packet.AppendChild(packet2)
		return packet

	case *ModifyResponse:
		var res = lr.(*ModifyResponse)
		packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
		packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagInteger, uint64(res.MessageID), "MessageID"))
		packet2 := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationModifyResponse, nil, "Modify response")
		packet2.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagEnumerated, uint64(res.ResultCode), "ResultCode"))
		packet2.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, string(res.MatchedDN), "MatchedDN"))
		packet2.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, res.DiagnosticMessage, "DiagnosticMessage"))
		packet.AppendChild(packet2)
		return packet

	case *CompareResponse:
		var res = lr.(*CompareResponse)
		packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
		packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagInteger, uint64(res.MessageID), "MessageID"))
		packet2 := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationCompareResponse, nil, "Compare response")
		packet2.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagEnumerated, uint64(res.ResultCode), "ResultCode"))
		packet2.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, string(res.MatchedDN), "MatchedDN"))
		packet2.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, res.DiagnosticMessage, "DiagnosticMessage"))
		packet.AppendChild(packet2)
		return packet
	case *ldapResult:
		res := lr.(*ldapResult)
		packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
		packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagInteger, uint64(res.MessageID), "MessageID"))
		packet2 := ber.Encode(ber.ClassApplication, ber.TypeConstructed, 0, nil, "Common")
		packet2.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimative, ber.TagEnumerated, uint64(res.ResultCode), "ResultCode"))
		packet2.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, string(res.MatchedDN), "MatchedDN"))
		packet2.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimative, ber.TagOctetString, res.DiagnosticMessage, "DiagnosticMessage"))
		packet.AppendChild(packet2)
		return packet

	default:
		log.Printf("newMessagePacket :: unexpected type %T", v)
	}
	return nil
}
