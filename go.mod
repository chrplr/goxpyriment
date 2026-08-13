module github.com/chrplr/goxpyriment

go 1.25.7

require (
	github.com/Zyko0/go-sdl3 v0.1.1
	github.com/ebitengine/purego v0.10.0
	github.com/funatsufumiya/go-gv-video v0.0.2
	github.com/gen2brain/mpeg v0.5.0
	github.com/goburrow/modbus v0.1.0
	github.com/pierrec/lz4/v4 v4.1.26
	go.bug.st/serial v1.6.4
	golang.org/x/sys v0.41.0
)

require (
	github.com/Zyko0/purego-gen v0.0.0-20250727121216-3bcd331a1e0c // indirect
	github.com/creack/goselect v0.1.3 // indirect
	github.com/goburrow/serial v0.1.0 // indirect
	github.com/robroyd/dds v0.0.0-20221227152439-75471f84d293 // indirect
)

replace github.com/Zyko0/go-sdl3 => github.com/chrplr/go-sdl3-wasm v0.1.2
