/*
Command gxc is a simple cross-compiling frontend for go.

gxc does cross-platform compilation by setting $GOOS, $GOARCH, and $CGO_ENABLED for
each target platform, and then executing "make.bash" followed by "go build"

It is inspired by golang-crosscompile and is similar to goxc

golang-crosscompile: http://github.com/davecheney/golang-crosscompile

goxc: http://github.com/laher/goxc

Install

     go get github.com/robertkrimen/gxc/gxc

Usage

     Usage: gxc ...                                                                     
                                                                                        
         -bashrc=false: Emit bash aliases: go-all, go-build-all, go-linux-386, ...      
         -exe=false: Add an .exe extension to files built for windows/*                 
         -stash="": Directory to deposit built files into                               
         -target="": The platforms to target (linux, windows/386, etc.)                 
                                                                                        
       list                                                                             
         List available platforms and status                                            
                                                                                        
       build [options]                                                                  
         Run "go build -o <name> [options]" for each platform                           
         The name is of the format <command/package>-<platform>                         
         Options are passed through to "go build"                                       
                                                                                        
       go [options]                                                                     
         Run "go [options]" for each platform                                           
         Options are passed through to "go"                                             
                                                                                        
       setup [options] [platform]                                                       
         Run make.bash for the specified platform (or every platform if none given)     
                                                                                        
         -force=false: Force make.bash to run, even if it already has                   
         -quiet=false: Quiet make.bash (redirect stdout/stderr > nil)                   
         -verbose=false: Pass make.bash output to stdout/stderr (instead of logging)    
                                                                                        
           # Build the current command/package for every platform                       
           gxc build                                                                    
                                                                                        
           # Build the current command/package for linux: (linux/amd64, linux/386)      
           gxc build-linux                                                              
                                                                                        
           # Build the command "xyzzy" for windows:                                     
           gxc build-windows xyzzy                                                      
                                                                                        
           # Build the command "xyzzy" for windows, linux, and darwin/386:              
           gxc -target="windows linux darwin/386" build xyzzy                           
                                                                                        
           # Run "go env" for each platform (contrived)                                 
           gxc go env                                                                   
                                                                                        
           # Setup bash aliases                                                         
           eval `gxc --bashrc`                                                          
*/
package main

// TODO Check for presence of gcc
// TODO Emit results as JSON for machine consumption?

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var (
	goRoot      = ""
	goHostMajor = ""
	goHostMinor = ""
)

var (
	matchKeyValue        = regexp.MustCompile(`(?m)^(?:set )?([^=]+)=(.*)$`)
	matchQuote           = regexp.MustCompile(`^(?:"(.*)")|(?:'(.*)')`)
	matchPlatformQuery   = regexp.MustCompile(`^([0-9a-z*]+)(?:[/\-_]([0-9a-z*]+))?$`)
	matchCompoundCommand = regexp.MustCompile(`^(setup|build|go)-([0-9a-z\-]+)$`)
	matchBuiltPackage    = regexp.MustCompile(`(?m)^#\s*\n^#\s*(.*)\s*\n^#\s*\n`)
)

var (
	platformUnix = _hostPlatform{
		runGo:     "go",
		buildAll:  "all.bash",
		buildMake: "make.bash",
	}

	platformWindows = _hostPlatform{
		runGo:     "go.exe",
		buildAll:  "all.bat",
		buildMake: "make.bat",
	}

	hostPlatform = platformUnix
)

type _hostPlatform struct {
	runGo     string
	buildAll  string
	buildMake string
}

var (
	flag_target = flag.String("target", "", "The platforms to target (linux, windows/386, etc.)")
	flag_bashrc = flag.Bool("bashrc", false, "Emit bash aliases: go-all, go-build-all, go-linux-386, ...")
	flag_exe    = flag.Bool("exe", false, "Add an .exe extension to files built for windows/*")
	flag_stash  = flag.String("stash", "", "Directory to deposit built files into")
	flag_quiet  = false // _GXC_QUIET
)

var (
	setupFlag         = flag.NewFlagSet("setup", flag.ExitOnError)
	setupFlag_force   = false
	setupFlag_verbose = false
	setupFlag_quiet   = false
	_                 = func() byte {
		setupFlag.BoolVar(&setupFlag_force, "force", setupFlag_force, "Force make.bash to run, even if it already has")
		setupFlag.BoolVar(&setupFlag_force, "f", setupFlag_force, string(0))
		setupFlag.BoolVar(&setupFlag_verbose, "verbose", setupFlag_verbose, "Pass make.bash output to stdout/stderr (instead of logging)")
		setupFlag.BoolVar(&setupFlag_verbose, "v", setupFlag_verbose, string(0))
		setupFlag.BoolVar(&setupFlag_quiet, "quiet", setupFlag_quiet, "Quiet make.bash (redirect stdout/stderr > nil)")
		setupFlag.BoolVar(&setupFlag_quiet, "q", setupFlag_quiet, string(0))
		return 0
	}()
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()

	fmt.Fprintf(os.Stderr, kilt.GraveTrim(`

 list
  List available platforms and status

 build [options]
  Run "go build -o <name> [options]" for each platform
  The name is of the format <command/package>-<platform>
  Options are passed through to "go build"

 go [options]
  Run "go [options]" for each platform
  Options are passed through to "go"
    
 setup [options] [platform]
  Run make.bash for the specified platform (or every platform if none given)

    `))
	kilt.PrintDefaults(setupFlag)

	fmt.Fprintf(os.Stderr, kilt.GraveTrim(`

    # Build the current command/package for every platform
    gxc build  

    # Build the current command/package for linux: (linux/amd64, linux/386)
    gxc build-linux

    # Build the command "xyzzy" for windows:
    gxc build-windows xyzzy

    # Build the command "xyzzy" for windows, linux, and darwin/386:
    gxc -target="windows linux darwin/386" build xyzzy

    # Run "go env" for each platform (contrived)
    gxc go env

    # Setup bash aliases
    eval %s

    `), "`gxc --bashrc`")
}

// go/src/pkg/runtime/defs*.h
// #!/bin/bash
// # Copyright 2012 The Go Authors. All rights reserved.
// # Use of this source code is governed by a BSD-style
// # license that can be found in the LICENSE file.
//
// # support functions for go cross compilation
//
// PLATFORMS="darwin/386 darwin/amd64 freebsd/386 freebsd/amd64 freebsd/arm linux/386 linux/amd64 linux/arm windows/386 windows/amd64"
//
// eval "$(go env)"
//
// function cgo-enabled {
// 	if [ "$1" = "${GOHOSTOS}" ]; then
// 		if [ "${GOHOSTOS}" != "freebsd/arm" ]; then
// 			echo 1
// 		else
// 			# cgo is not freebsd/arm
// 			echo 0
// 		fi
// 	else
// 		echo 0
// 	fi
// }
//
// function go-alias {
// 	GOOS=${1%/*}
// 	GOARCH=${1#*/}
// 	eval "function go-${GOOS}-${GOARCH} { (CGO_ENABLED=$(cgo-enabled ${GOOS} ${GOARCH}) GOOS=${GOOS} GOARCH=${GOARCH} go \$@ ) }"
// }
//
// function go-crosscompile-build {
// 	GOOS=${1%/*}
// 	GOARCH=${1#*/}
// 	cd ${GOROOT}/src ; CGO_ENABLED=$(cgo-enabled ${GOOS} ${GOARCH}) GOOS=${GOOS} GOARCH=${GOARCH} ./make.bash --no-clean 2>&1
// }
//
// function go-crosscompile-build-all {
// 	FAILURES=""
// 	for PLATFORM in $PLATFORMS; do
// 		CMD="go-crosscompile-build ${PLATFORM}"
// 		echo "$CMD"
// 		$CMD || FAILURES="$FAILURES $PLATFORM"
// 	done
// 	if [ "$FAILURES" != "" ]; then
// 	    echo "*** go-crosscompile-build-all FAILED on $FAILURES ***"
// 	    return 1
// 	fi
// }
//
// function go-all {
// 	FAILURES=""
// 	for PLATFORM in $PLATFORMS; do
// 		GOOS=${PLATFORM%/*}
// 		GOARCH=${PLATFORM#*/}
// 		CMD="go-${GOOS}-${GOARCH} $@"
// 		echo "$CMD"
// 		$CMD || FAILURES="$FAILURES $PLATFORM"
// 	done
// 	if [ "$FAILURES" != "" ]; then
// 	    echo "*** go-all FAILED on $FAILURES ***"
// 	    return 1
// 	fi
// }
//
// function go-build-all {
// 	FAILURES=""
// 	for PLATFORM in $PLATFORMS; do
// 		GOOS=${PLATFORM%/*}
// 		GOARCH=${PLATFORM#*/}
// 		OUTPUT=`echo $@ | sed 's/\.go//'`
// 		CMD="go-${GOOS}-${GOARCH} build -o $OUTPUT-${GOOS}-${GOARCH} $@"
// 		echo "$CMD"
// 		$CMD || FAILURES="$FAILURES $PLATFORM"
// 	done
// 	if [ "$FAILURES" != "" ]; then
// 	    echo "*** go-build-all FAILED on $FAILURES ***"
// 	    return 1
// 	fi
// }
//
// for PLATFORM in $PLATFORMS; do
// 	go-alias $PLATFORM
// done
//
// unset -f go-alias

func environment(override ...string) []string {
	matchExclude := regexp.MustCompile(`^(GO(?:ARCH|OS)|CGO_ENABLED)=`)
	// This tmp will be the current environment (excluding matchExclude)
	tmp := []string(nil)
	for _, value := range os.Environ() {
		if matchExclude.MatchString(value) {
			continue
		}
		tmp = append(tmp, value)
	}
	return append(tmp, override...)
}

type _platform struct {
	major string // Operating System ($GOOS)
	minor string // Architecture ($GOARCH)
}

type _failure struct {
	platform _platform
}

func (self _platform) String() string {
	return self.major + "/" + self.minor
}

func (self _platform) native() bool {
	return self.major == goHostMajor && self.minor == goHostMinor
}

// ${GOROOT}/pkg/${GOOS}_${GOARCH}/.gxc
func (self _platform) builtFile() string {
	return filepath.Join(goRoot, "pkg", self.major+"_"+self.minor, ".gxc")
}

func (self _platform) match(query string) bool {
	switch query {
	case "", "*", "all":
		return true
	}
	if match := matchPlatformQuery.FindStringSubmatch(query); match != nil {
		targetMajor, targetMinor := match[1], match[2]
		switch targetMajor {
		case "", "*":
		default:
			if targetMajor != self.major {
				return false
			}
		}
		switch targetMinor {
		case "", "*":
		default:
			if targetMinor != self.minor {
				return false
			}
		}
		return true
	}
	return false
}

func (self _platform) isReady() bool {
	_, err := os.Stat(self.builtFile())
	return err == nil
}

func (self _platform) cgoFlag() string {
	if self.native() {
		return "CGO_ENABLED=1"
	}
	return "CGO_ENABLED=0"
}

func (self _platform) buildCompiler(stdout io.Writer, stderr io.Writer) error {

	cmd := exec.Command(filepath.Join(goRoot, "src", hostPlatform.buildMake), "--no-clean")
	cmd.Dir = filepath.Dir(cmd.Path)
	cmd.Env = environment(
		"GOOS="+self.major,
		"GOARCH="+self.minor,
		self.cgoFlag(), // CGO_ENABLED=
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		return err
	}
	{
		path := self.builtFile()
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
		return err
	}
}

// darwin/386
// darwin/amd64
// freebsd/386
// freebsd/amd64
// linux/386
// linux/amd64
// linux/arm
// windows/386
// windows/amd64

var registry []_platform

func firstTimeSetup(target []_platform) {
	for _, platform := range target {
		// If at least one platform is ready, then return
		// Assume the user has already tried to setup before
		// (We do not want to keep trying to run a slow, broken make.bash)
		if platform.isReady() {
			return
		}
	}
	doSetup(target, nil)
}

func doSetup(target []_platform, arguments []string) (failure []_failure) {
	setupFlag.Parse(arguments)
	arguments = setupFlag.Args()
	if len(arguments) > 0 {
		// e.g. $ gxc setup windows linux-amd64 freebsd-386
		target = nil
		for _, query := range arguments {
			target = append(target, platformQuery(query)...)
		}
	}

	// A bulk setup is doing more than one
	bulk := len(target) > 1

	for _, platform := range target {
		if bulk && platform.native() {
			continue
		}
		if setupFlag_force {
			os.Remove(platform.builtFile())
		}
		if platform.isReady() {
			fmt.Fprintf(os.Stderr, "+ %s\n", platform)
			continue
		}

		var stdout, stderr io.Writer
		var log *os.File
		emit := ""
		if setupFlag_verbose {
			emit = "-"
			stdout = os.Stdout
			stderr = os.Stderr
		} else if setupFlag_quiet {
		} else {
			log, _ = ioutil.TempFile("", "make."+platform.major+"-"+platform.minor+".log.")
			if log != nil {
				defer log.Close()
				stdout = log
				stderr = log
				emit = log.Name()
			}
		}
		fmt.Fprintf(os.Stderr, "- %s\n", platform)
		fmt.Fprintf(os.Stderr, "# Building platform: %s (%s)\n", platform, emit)
		err := platform.buildCompiler(stdout, stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "! %s: %s\n", platform, err)
			failure = append(failure, _failure{
				platform: platform,
			})
		} else {
			if log != nil {
				if false {
					// Should we do this?
					os.Remove(log.Name())
				}
			}
			fmt.Fprintf(os.Stderr, "+ %s\n", platform)
		}
	}
	return failure
}

func findBuiltName(arguments []string) (name string) {
	name = "build"
	cmd := exec.Command("go", append([]string{"build", "-n"}, arguments...)...)
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gxc: unable to guess built name: %v\n", err)
		return
	}
	if match := matchBuiltPackage.FindAllSubmatch(output, -1); match != nil {
		pkg := string(match[len(match)-1][1])
		name = filepath.Base(pkg)
	} else {
		fmt.Fprintf(os.Stderr, "gxc: unable to guess built name\n")
	}
	return
}

func doBuild(target []_platform, arguments []string) (failure []_failure) {
	firstTimeSetup(target)
	name := findBuiltName(arguments)

	stash := *flag_stash
	if stash != "" {
		stash = filepath.Clean(stash)
		os.MkdirAll(stash, 0777) // Ignore error, "go build" will squawk below
	}

	for _, platform := range target {
		if !platform.isReady() {
			continue
		}
		output := strings.Join([]string{name, platform.major, platform.minor}, "-")
		if stash != "" {
			output = filepath.Join(stash, output)
		}
		if *flag_exe && platform.major == "windows" {
			output += ".exe"
		}
		fmt.Fprintf(os.Stderr, "# Build: %s\n", output)
		cmd := exec.Command("go", append([]string{"build", "-o", output}, arguments...)...)
		cmd.Env = environment(
			"GOOS="+platform.major,
			"GOARCH="+platform.minor,
			platform.cgoFlag(), // CGO_ENABLED=
		)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			failure = append(failure, _failure{
				platform: platform,
			})
			if !flag_quiet {
				fmt.Fprintf(os.Stderr, "! %s: %s\n", platform, err)
			}
		}
	}
	return failure
}

func doGo(target []_platform, arguments []string) (failure []_failure) {
	firstTimeSetup(target)
	for _, platform := range target {
		if !platform.isReady() {
			continue
		}
		cmd := exec.Command("go", arguments...)
		cmd.Env = environment(
			"GOOS="+platform.major,
			"GOARCH="+platform.minor,
			platform.cgoFlag(), // CGO_ENABLED=
		)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			failure = append(failure, _failure{
				platform: platform,
			})
			if !flag_quiet {
				fmt.Fprintf(os.Stderr, "! %s: %s\n", platform, err)
			}
		}
	}
	return failure
}

func platformQuery(query string) []_platform {
	switch query {
	case "", "all":
		return registry
	}
	found := []_platform{}
	for _, query := range strings.Fields(query) {
		for _, platform := range registry {
			if platform.match(query) {
				found = append(found, platform)
			}
		}
	}
	return found
}

func platformMatch(query []string) ([]_platform, []string) {
	index := 0
	match := []_platform{}
	for _, query := range query {
		found := platformQuery(query)
		if len(found) == 0 {
			break
		}
		match = append(match, found...)
		index += 1
	}
	if index < len(query) {
		query = query[index:]
	} else {
		query = []string(nil)
	}
	return match, query
}

func bashrc() {
	fmt.Fprintf(os.Stdout, kilt.GraveTrim(`
GXC_TARGET=();

function go-crosscompile-build {
_GXC_QUIET=1 gxc setup "$@"
};

function go-build-all {
_GXC_QUIET=1 gxc -target="$GXC_TARGET" build "$@"
};

function go-all {
_GXC_QUIET=1 gxc -target="$GXC_TARGET" go "$@"
};

    `))

	for _, platform := range platformQuery(*flag_target) {
		fmt.Fprintf(os.Stdout, kilt.GraveTrim(`
GXC_TARGET+=("%s");
function go-%s-%s {
_GXC_QUIET=1 gxc -target="%s" go "$@"
};

        `), platform, platform.major, platform.minor, platform)
	}
	os.Exit(0)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if os.ExpandEnv("$_GXC_QUIET") == "1" {
		flag_quiet = true
	}
	if runtime.GOOS == "windows" {
		hostPlatform = platformWindows
	}
	err := func() error {
		{
			// We want to have a pure go environment, to fix any fiddling
			// go env:
			output, err := exec.Command("go", "env").Output()
			if err != nil {
				return err
			}
			if match := matchKeyValue.FindAllSubmatch(output, -1); match != nil {
				for _, match := range match[1:] {
					key, value := string(match[1]), string(match[2])
					if match := matchQuote.FindStringSubmatch(value); match != nil {
						value = match[1]
					}
					switch key {
					case "GOROOT":
						goRoot = value
					case "GOHOSTOS":
						goHostMajor = value
					case "GOHOSTARCH":
						goHostMinor = value
					}
					os.Setenv(key, value)
				}
			} else {
				return fmt.Errorf(`missing Go environment (go env)`)
			}
		}

		{
			file, err := os.Open(filepath.Join(goRoot, "pkg"))
			if err == nil {
				files, err := file.Readdirnames(-1)
				if err != nil {
					fmt.Fprintln(os.Stderr, "gxc: problem populating platform registry", err)
				}
				for _, name := range files {
					if strings.HasPrefix(name, "defs_") && strings.HasSuffix(name, ".h") {
						name = name[5 : len(name)-2] // defs_*.h
						index := strings.Index(name, "_")
						major := name[0:index]
						minor := name[index+1:]
						registry = append(registry, _platform{
							major: major,
							minor: minor,
						})
					}
				}

			} else {
				fmt.Fprintln(os.Stderr, "gxc: unable to populate platform registry", err)
			}
		}

		if *flag_bashrc {
			bashrc()
		}

		command := flag.Arg(0)
		query := ""
		// A compound command is something like "build-$platform": "build-linux-386", "build-windows", etc.
		if match := matchCompoundCommand.FindStringSubmatch(command); match != nil {
			command = match[1]
			query = match[2]
		} else {
			query = *flag_target
		}

		if command == "" {
			usage()
			os.Exit(2)
		} else {
			arguments := flag.Args()[1:]
			failure := []_failure{}
			target := []_platform{}
			switch command {
			case "build":
				found := false
				if query == "" {
					target, arguments = platformMatch(arguments)
					found = len(target) > 0
				}
				if !found {
					target = platformQuery(query)
				}
				failure = doBuild(target, arguments)
			case "setup":
				target = platformQuery(query)
				failure = doSetup(target, arguments)
			case "go":
				target = platformQuery(query)
				failure = doGo(target, arguments)
			case "list":
				for _, platform := range registry {
					ready := "-"
					if platform.isReady() {
						ready = "+"
					}
					fmt.Fprintf(os.Stdout, "%s %s\n", ready, platform)
				}
			case "bashrc":
				bashrc()
			default:
				return fmt.Errorf("invalid command: %s", command)
			}
			if len(failure) != 0 {
				platform := []string{}
				for _, failure := range failure {
					platform = append(platform, failure.platform.String())
				}
				return fmt.Errorf("%s failure (%d): %s", command, len(failure), strings.Join(platform, " "))
			}
		}

		return nil
	}()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gxc:", err)
		os.Exit(1)
	}
}
