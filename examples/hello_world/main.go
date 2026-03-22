package main

import (
	"fmt"
	"github.com/chrplr/goxpyriment/control"
)

func main() {
	fmt.Println("Start")
	exp := control.NewExperimentFromFlags("My First", control.Black, control.White, 32)
	fmt.Println("Init done")
	exp.Run(func() error {
		fmt.Println("Logic start")
		return control.EndLoop
	})
	fmt.Println("End")
}