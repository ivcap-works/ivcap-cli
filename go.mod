module github.com/reinventingscience/ivcap-client

go 1.18

require (
	cayp/api_gateway v0.0.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/jedib0t/go-pretty/v6 v6.3.1
	github.com/k0kubun/go-ansi v0.0.0-20180517002512-3bf9e2903213
	github.com/schollz/progressbar/v3 v3.9.0
	github.com/spf13/cobra v1.5.0
	go.uber.org/zap v1.23.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/dimfeld/httptreemux/v5 v5.4.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	goa.design/goa/v3 v3.7.5 // indirect
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa // indirect
	golang.org/x/sys v0.0.0-20220728004956-3c1f35247d10 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
)

replace cayp/api_gateway v0.0.0 => ../ivap-core/api_gateway

replace cayp/common v0.0.0 => ../ivap-core/common

replace cayp/metadata v0.0.0 => ../ivap-core/metadata
