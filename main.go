package main

import (
	"log"
	"os"

	"github.com/mitchellh/cli"
)

func main() {

	c := cli.NewCLI("app", "1.0.0")
	c.Args = os.Args[1:]
	c.Commands = map[string]cli.CommandFactory{
		"http": httpCommandFactory,
	}

	exitStatus, err := c.Run()
	if err != nil {
		log.Println(err)
	}

	os.Exit(exitStatus)
}

type Point struct {
	Value     float64
	Metric    string
	Tags      map[string]string
	Timestamp int64
}
