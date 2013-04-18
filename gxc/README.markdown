# gxc
--
Command gxc is a simple cross-compiling frontend for go.

gxc does cross-platform compilation by setting $GOOS, $GOARCH, and $CGO_ENABLED for
each target platform, and then executing "make.bash" followed by "go build"

It is inspired by golang-crosscompile and is similar to goxc

golang-crosscompile: http://github.com/davecheney/golang-crosscompile

goxc: http://github.com/laher/goxc

### Install

     go get github.com/robertkrimen/gxc/gxc

### Usage

     Usage: gxc ...

         -bashrc=false: Emit bash aliases: go-all, go-build-all, go-linux-386, ...
         -exe=false: Add an .exe extension to files built for windows/*
         -stash="": Directory to deposit built files into
         -target="": The platforms to target (linux, windows/386, etc.)

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

         -force=false: Force make.bash to run, even if it already has
         -quiet=false: Quiet make.bash (redirect stdout/stderr > nil)
         -verbose=false: Pass make.bash output to stdout/stderr (instead of logging)

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
           eval `gxc --bashrc`

--
**godocdown** http://github.com/robertkrimen/godocdown
