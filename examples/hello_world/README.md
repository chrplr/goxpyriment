# Hello World

A minimal standalone goxpyriment example that can be copied outside the repository and built without the `go.work` workspace. It shows a coloured rectangle on screen and waits for a key press.

Use this as a starting point if you want to develop an experiment in its own separate Go module.

## Prerequisites

- Go 1.25+

## Running inside the repository

```bash
go run main.go
go run main.go -w   # windowed
```

## Running as a standalone module (outside the repo)

```bash
cp -r examples/hello_world ~/my-experiment
cd ~/my-experiment
go mod init my-experiment
go mod tidy
go run main.go
```

## Building a binary

```bash
go build -o hello_goxpy .
./hello_goxpy
```

## Controls

Press any key to exit.
