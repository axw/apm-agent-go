package main

import (
	"fmt"
)

func rewriteSource(workdir, packagePath string, files []string) {
	// TODO(axw) get the import path, not just "main".
	// The path doesn't get passed into "go tool compile".
	fmt.Println(packagePath)
}
