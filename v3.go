package paseto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha512"

	"aidanwoods.dev/go-paseto/internal/encoding"
	"aidanwoods.dev/go-paseto/internal/random"
	"github.com/pkg/errors"
)

func v3LocalEncrypt(p packet, key V3SymmetricKey, implicit []byte, unitTestNonce []byte) message {
	var nonce [32]byte
	random.UseProvidedOrFillBytes(unitTestNonce, nonce[:])

	encKey, authKey, nonce2 := key.split(nonce)

	blockCipher, err := aes.NewCipher(encKey[:])
	if err != nil {
		panic("Cannot construct cipher")
	}

	cipherText := make([]byte, len(p.content))
	cipher.NewCTR(blockCipher, nonce2[:]).XORKeyStream(cipherText, p.content)

	header := []byte(V3Local.Header())

	preAuth := encoding.Pae(header, nonce[:], cipherText, p.footer, implicit)

	hm := hmac.New(sha512.New384, authKey[:])
	if _, err := hm.Write(preAuth); err != nil {
		panic(err)
	}
	var tag [48]byte
	copy(tag[:], hm.Sum(nil))

	return newMessageFromPayload(v3LocalPayload{nonce, cipherText, tag}, p.footer)
}

func v3LocalDecrypt(msg message, key V3SymmetricKey, implicit []byte) (packet, error) {
	payload, ok := msg.p.(v3LocalPayload)
	if msg.header() != V3Local.Header() || !ok {
		return packet{}, errors.Errorf("Cannot decrypt message with header: %s", msg.header())
	}

	nonce, cipherText, givenTag := payload.nonce, payload.cipherText, payload.tag
	encKey, authKey, nonce2 := key.split(nonce)

	header := []byte(msg.header())

	preAuth := encoding.Pae(header, nonce[:], cipherText, msg.footer, implicit)

	hm := hmac.New(sha512.New384, authKey[:])
	if _, err := hm.Write(preAuth); err != nil {
		panic(err)
	}
	var expectedTag [48]byte
	copy(expectedTag[:], hm.Sum(nil))

	if !hmac.Equal(expectedTag[:], givenTag[:]) {
		var p packet
		return p, errors.Errorf("Bad message authentication code")
	}

	blockCipher, err := aes.NewCipher(encKey[:])
	if err != nil {
		panic("Cannot construct cipher")
	}

	plainText := make([]byte, len(cipherText))
	cipher.NewCTR(blockCipher, nonce2[:]).XORKeyStream(plainText, cipherText)

	return packet{plainText, msg.footer}, nil
}
