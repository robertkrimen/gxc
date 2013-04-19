/*
Package gxc is a simple cross-compiling frontend for go.

gxc does cross-platform compilation by setting $GOOS, $GOARCH, and $CGO_ENABLED for
each target platform, and then executing "make.bash" followed by "go build"

This is a placeholder package, the actual command is:
http://github.com/robertkrimen/gxc/gxc

    # Build the current command/package for every platform                     
    gxc build                                                                  
                                                                               
    # Build the current command/package for linux: (linux/amd64, linux/386)    
    gxc build-linux                                                            
                                                                               
    # Build the command "xyzzy" for windows:                                   
    gxc build-windows xyzzy                                                    
                                                                               
    # Build the command "xyzzy" for windows, linux, and darwin/386:            
    gxc -target="windows linux darwin/386" build xyzzy                         
                                                                               
    # Run "go env" for each platform (contrived)                               
    gxc go env                                                                 
                                                                               
    # Setup bash aliases                                                       
    eval `gxc --bashrc`                                                        
*/
package gxc

import (
	_ "github.com/robertkrimen/gxc/gxc"
)
