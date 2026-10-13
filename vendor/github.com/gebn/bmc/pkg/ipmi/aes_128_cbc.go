package ipmi

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// AES128CBC implements the AES-128-CBC confidentiality algorithm specified in
// section 13.29 of IPMI v2.0. It is a payload layer type, used to encrypt and
// decrypt IPMI messages. Note the default instance is not usable: use
// NewAES128CBC() to create one.
type AES128CBC struct {
	layers.BaseLayer

	// cipher is an instance of AES loaded with the first 128 bits of K2 as the
	// key. This field is of course not included in the packet data, and must be
	// set before serialising or decoding any packets.
	cipher cipher.Block

	// lack of IV field is deliberate so implementations cannot forget to set it
}

func NewAES128CBC(k2 [16]byte) (*AES128CBC, error) {
	c, err := aes.NewCipher(k2[:])
	if err != nil {
		return nil, err
	}
	return &AES128CBC{
		cipher: c,
	}, nil
}

func (*AES128CBC) LayerType() gopacket.LayerType {
	return layerTypeAES128CBC
}

func (a *AES128CBC) CanDecode() gopacket.LayerClass {
	return a.LayerType()
}

func (a *AES128CBC) NextLayerType() gopacket.LayerType {
	return LayerTypeMessage
}

// DecodeFromBytes decodes an AES-128-CBC confidentiality layer. Although a
// precise error is returned, care should be taken when displaying this, as it
// can lead to a padding oracle attack (essentially, don't reveal the difference
// between "decryption failed" and "invalid padding" errors).
func (a *AES128CBC) DecodeFromBytes(data []byte, _ gopacket.DecodeFeedback) error {
	if len(data) < a.cipher.BlockSize()+1 || len(data)%a.cipher.BlockSize() != 0 {
		return fmt.Errorf(
			"AES payload must be at least %v bytes and have an overall length divisible by %v, got length of %v",
			a.cipher.BlockSize()+1, a.cipher.BlockSize(), len(data))
	}

	iv := data[:a.cipher.BlockSize()]
	a.BaseLayer.Contents = iv
	mode := cipher.NewCBCDecrypter(a.cipher, iv)
	// technically we break the rules here by modifying the payload, however
	// ciphertext is useless on its own, so we don't mind
	mode.CryptBlocks(data[a.cipher.BlockSize():], data[a.cipher.BlockSize():])

	padBytes := uint8(data[len(data)-1])
	// table 13-20 of the spec says the confidentiality pad length ranges from
	// 0 to 15 bytes if using AES, but we may receive 16 bytes if the BMC's
	// implementation of AES in CBC mode requires a minimum of one pad byte
	// (which is how OpenSSL works) and the message is already aligned.
	if padBytes > uint8(a.cipher.BlockSize()) {
		return fmt.Errorf("invalid number of pad bytes: %v", padBytes)
	}
	padStart := len(data) - int(padBytes) - 1
	// table 13-20 of the spec says we should check the pad
	v := uint8(1)
	for i := padStart; i < padStart+int(padBytes); i++ {
		if data[i] != v {
			return fmt.Errorf("invalid pad byte: offset %v (%v within payload) should have value %v, but has value %v", v-1, i, v, data[i])
		}
		v++
	}
	a.BaseLayer.Payload = data[a.cipher.BlockSize():padStart]
	return nil
}

func (a *AES128CBC) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	// write confidentiality trailer so it is there to encrypt next
	padLength := (a.cipher.BlockSize() - 1) - (len(b.Bytes()) % a.cipher.BlockSize())
	trailer, err := b.AppendBytes(padLength + 1)
	if err != nil {
		return err
	}
	for i := 0; i < padLength; i++ {
		trailer[i] = uint8(i + 1) // 0x01, 0x02, 0x03 etc.
	}
	trailer[padLength] = uint8(padLength)

	toEncrypt := b.Bytes() // includes confidentiality trailer

	// secure random IV for confidentiality header
	iv, err := b.PrependBytes(a.cipher.BlockSize())
	if err != nil {
		return err
	}
	if _, err := rand.Read(iv); err != nil {
		return err
	}

	// encrypt everything after IV
	mode := cipher.NewCBCEncrypter(a.cipher, iv)
	mode.CryptBlocks(toEncrypt, toEncrypt)
	return nil
}
