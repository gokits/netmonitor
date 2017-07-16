package proto

type Hello struct {
	Ver       string
	Challenge string
}

type HelloRsp struct {
	CheckSum  []byte
	Challenge string
}
