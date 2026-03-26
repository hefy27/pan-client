package aliyundrive

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"

	"github.com/dustinxie/ecc"
)

type deviceState struct {
	deviceID   string
	signature  string
	privateKey *ecdsa.PrivateKey
}

var (
	stateMu sync.Mutex
	states  = map[string]*deviceState{}
)

func getOrCreateState(userID string) *deviceState {
	stateMu.Lock()
	defer stateMu.Unlock()
	if s, ok := states[userID]; ok {
		return s
	}
	deviceID := hashSHA256(userID)
	pk, _ := newPrivateKeyFromHex(deviceID)
	s := &deviceState{
		deviceID:   deviceID,
		privateKey: pk,
	}
	states[userID] = s
	return s
}

func hashSHA256(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func newPrivateKeyFromHex(hexStr string) (*ecdsa.PrivateKey, error) {
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	p256k1 := ecc.P256k1()
	x, y := p256k1.ScalarBaseMult(data)
	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: p256k1,
			X:     x,
			Y:     y,
		},
		D: new(big.Int).SetBytes(data),
	}, nil
}

func publicKeyToHex(pub *ecdsa.PublicKey) string {
	x := pub.X.Bytes()
	for len(x) < 32 {
		x = append([]byte{0}, x...)
	}
	y := pub.Y.Bytes()
	for len(y) < 32 {
		y = append([]byte{0}, y...)
	}
	return hex.EncodeToString(append(x, y...))
}

func signData(state *deviceState, userID string) {
	singdata := fmt.Sprintf("%s:%s:%s:%d", SECP_APP_ID, state.deviceID, userID, 0)
	hash := sha256.Sum256([]byte(singdata))
	data, _ := ecc.SignBytes(state.privateKey, hash[:], ecc.RecID|ecc.LowerS)
	state.signature = hex.EncodeToString(data)
}

func createSession(state *deviceState, refreshToken, accessToken, userID string) error {
	signData(state, userID)
	// The session creation would normally send a request, but we pre-sign here.
	// Actual session creation is handled in the request wrapper when needed.
	_ = rand.Reader
	return nil
}
