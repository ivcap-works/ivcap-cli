module github.com/reinventingscience/ivcap-client

go 1.18

require (
	cayp/api_gateway v0.0.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/google/uuid v1.3.0
	github.com/jedib0t/go-pretty/v6 v6.3.1
	github.com/spf13/cobra v1.4.0
	go.uber.org/zap v1.21.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/dimfeld/httptreemux/v5 v5.3.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/testify v1.7.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	goa.design/goa/v3 v3.4.3 // indirect
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad // indirect
)

replace cayp/api_gateway v0.0.0 => ../ivap-core/api_gateway

replace cayp/common v0.0.0 => ../ivap-core/common

replace cayp/metadata v0.0.0 => ../ivap-core/metadata
