package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	realGoEnv    = "REALGO"
	apmGoModeEnv = "APM_GO_MODE"
)

var (
	realGo    = os.Getenv(realGoEnv)
	apmGoMode = os.Getenv(apmGoModeEnv)
)

func main() {
	switch apmGoMode {
	case "", "go":
		mainGo()
	case "toolexec":
		mainToolexec()
	default:
		log.Fatalf("unknown mode %q", apmGoMode)
	}
}

func mainGo() {
	goCmd := "go"
	if os.Args[0] == "go" {
		log.Fatalf(
			"apmgo wrapper was executed as %q, and $%s not specified",
			os.Args[0], realGoEnv,
		)
	} else if realGo != "" {
		goCmd = realGo
	}
	env := os.Environ()[:]
	args := os.Args[1:]
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "build", "install", "get":
			args = updateBuildArgs(args[0], args[1:])
			env = append(env, "GOCACHE=off")
			env = append(env, apmGoModeEnv+"=toolexec")
		}
	}
	execve(goCmd, args, env)
}

// updateBuildArgs takes the arguments for a "go build"-like command,
// and updates the arguments for instrumentation.
func updateBuildArgs(command string, args []string) []string {
	bf, packages := splitBuildFlags(command, args)
	new := []string{command}

	// Funnel all go build/install/get commands through
	// this executable.
	//
	// TODO(axw) check if 'args' contains -toolexec, and
	// store it in an environment variable and execute
	// that in mainToolexec.
	new = append(new, "-toolexec", os.Args[0])

	installsuffix := "apmgo"
	if bf.installsuffix != "" {
		installsuffix = bf.installsuffix + "_" + installsuffix
	}
	new = append(new, "-installsuffix", installsuffix)
	new = append(new, bf.args...)
	new = append(new, packages...)
	return new
}

func mainToolexec() {
	tool, args := os.Args[1], os.Args[2:]
	if filepath.Base(tool) == "compile" {
		args = updateCompileArgs(args)
	}
	// TODO(axw) pull wrapped -toolexec out of environment
	// variable, execute command with that.
	execve(tool, args, nil)
}

func updateCompileArgs(args []string) []string {
	// TODO(axw) support whitelist of packages to instrument.
	var packagePath, workdir string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-V") {
			// "go tool compile -V" should not be processed further.
			return args
		}
		switch arg {
		case "-o":
			workdir = filepath.Dir(args[i+1])
			i++
		case "-p":
			packagePath = args[i+1]
			i++
		}
	}
	if workdir == "" {
		log.Fatal("missing -o")
	}
	if packagePath == "" {
		log.Fatal("missing -p")
	}
	i := len(args)
	for ; i > 0; i-- {
		arg := args[i-1]
		if !strings.HasSuffix(arg, ".go") {
			break
		}
	}
	rewriteSource(workdir, packagePath, args[i:])
	// TODO(axw) optionally include original source code in build.
	return args
}

func execve(exe string, args, env []string) {
	cmd := exec.Command(exe, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = env
	err := cmd.Run()
	switch err := err.(type) {
	case nil:
		os.Exit(0)
	case *exec.ExitError:
		status, ok := err.Sys().(syscall.WaitStatus)
		if ok {
			os.Exit(status.ExitStatus())
		}
	}
	log.Fatal(err)
}

type buildFlags struct {
	args          []string
	installsuffix string
	toolexec      string
}

func splitBuildFlags(command string, args []string) (flags buildFlags, packages []string) {
	boolFlags := []string{"-a", "-n", "-race", "-msan", "-v", "-work", "-x", "-linkshared"}
	switch command {
	case "build":
		boolFlags = append(boolFlags, "-i")
	case "install":
		boolFlags = append(boolFlags, "-i")
	case "get":
		boolFlags = append(boolFlags, "-d", "-f", "-fix", "insecure", "-t", "-u")
	}
	bf := buildFlags{args: args[:]}
	i := 0
	for i < len(args) {
		n := bf.consume(args, i, boolFlags)
		if n == 0 {
			break
		}
		i += n
	}
	bf.args, packages = args[:i], args[i:]
	return bf, packages
}

func (bf *buildFlags) consume(args []string, i int, boolFlags []string) int {
	arg := args[i]
	if !strings.HasPrefix(arg, "-") {
		return 0
	}
	if arg == "--" {
		return 1
	}
	if strings.HasPrefix(arg, "--") {
		arg = arg[1:]
	}
	switch arg {
	case "-h", "-help":
		return 1
	}
	if pos := strings.IndexRune(arg, '='); pos > 0 {
		// -x=y
		bf.interpret(arg[:pos], arg[pos+1:])
		return 1
	}
	for _, f := range boolFlags {
		if arg == f {
			return 1
		}
	}
	// Assume non-bool flag, consume the next arg too.
	if len(args) <= i+1 {
		return 1
	}
	bf.interpret(arg, args[i+1])
	return 2
}

func (bf *buildFlags) interpret(k, v string) {
	switch k {
	case "-installsuffix":
		bf.installsuffix = v
	case "-toolexec":
		bf.toolexec = v
	}
}
