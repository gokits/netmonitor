package proto

import (
	"crypto/md5"
)

func AuthV1(token, challenge string) []byte {
	md := md5.New()
	md.Write([]byte(token))
	md.Write([]byte(challenge))
	return md.Sum(nil)
}
