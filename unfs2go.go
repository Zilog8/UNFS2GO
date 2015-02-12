package main

/*
#include <stdio.h>
#include "unfs3/daemon.h"
#include "unfs3/daemon.c"
*/
import "C"
import (
	"./minfs"
	"./osfs"
	"./shimfs"
	"./zipfs"
	"errors"
	"fmt"
"os"
)

var ns minfs.MinFS //filesystem being shared

func main() {

	args := os.Args[1:]
	
	tfs, err := parseArgs(args)

	if err != nil {
		fmt.Println("Error starting: ", err)
	}

	ns = tfs
		C.exports_parse(C.CString("/"), C.CString("rw"))
	C.start()
}

func parseArgs(args []string) (minfs.MinFS, error) {
	switch args[0] {
	case "-z":
		return zipfsPrep(args[1:])
	case "-o":
		return osfsPrep(args[1:])
	case "-s":
		return shimfsPrep(args[1:])
	default:
		return nil, errors.New("Not a recognized argument: " + args[0])
	}
}

func osfsPrep(args []string) (minfs.MinFS, error) {
	return osfs.New(args[0])
}

func zipfsPrep(args []string) (minfs.MinFS, error) {
	return zipfs.New(args[0])
}

func shimfsPrep(args []string) (minfs.MinFS, error) {
	sub, err := parseArgs(args)
	if err != nil {
		return nil, err
	}
	return shimfs.New("", 0, sub)
}
