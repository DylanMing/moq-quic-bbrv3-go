module github.com/DineshAdhi/moq-go

go 1.25

// 使用 quic-go-bbr 替换原版 quic-go
replace github.com/quic-go/quic-go => ../quic-go-bbr

require (
	github.com/quic-go/qpack v0.6.0
	github.com/quic-go/quic-go v0.45.1
	github.com/rs/zerolog v1.33.0
)

require (
	filippo.io/mkcert v1.4.4 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	golang.org/x/text v0.28.0 // indirect
	howett.net/plist v1.0.0 // indirect
	software.sslmate.com/src/go-pkcs12 v0.2.0 // indirect
)

require (
	github.com/google/uuid v1.6.0
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842
	golang.org/x/net v0.43.0
	golang.org/x/sys v0.35.0 // indirect
)
