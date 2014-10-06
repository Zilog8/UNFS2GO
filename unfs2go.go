package main

/*
#include <stdio.h>
#include "unfs3/daemon.h"
#include "unfs3/daemon.c"
*/
import "C"
import (
"os"
)

func main() {
	zipfilepath = os.Args[1]
	C.start()
}
