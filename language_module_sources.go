package main

import "github.com/electrikmilk/cherri/internal/language/modules"

func init() {
	source, err := stdFuncs.ReadFile("stdfunc.cherri")
	if err != nil {
		panic("preserved embedded standard function source is unavailable")
	}
	modules.RegisterStandardLibrary(string(source))
}
