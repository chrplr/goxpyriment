package main

import (
    "fmt"
    "github.com/Zyko0/go-sdl3/sdl"
    "github.com/Zyko0/go-sdl3/bin/binsdl"
)

func main() {
    fmt.Println("Loading binsdl")
    defer binsdl.Load().Unload()
    fmt.Println("Init")
    if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS | sdl.INIT_AUDIO); err != nil {
        panic(err)
    }
    fmt.Println("Init complete")
    fmt.Println("Opening Audio Device")
    dev, err := sdl.AUDIO_DEVICE_DEFAULT_PLAYBACK.OpenAudioDevice(nil)
    if err != nil {
        panic(err)
    }
    fmt.Println("Audio Device Opened:", dev)
}
