package main

import (
	"flag"
)

func main() {
	runA := flag.Bool("a", false, "run a")
	runB := flag.Bool("b", false, "run b")
	flag.Parse()
	if *runA {
		a()
	}
	if *runB {
		b()
	}
}
