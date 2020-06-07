# loopback filesystem

This folder contains code for a simple file system which reflects the file system calls to a loopback directory

Build the files inside main using:
go build loopback.go utilities.go fsNode.go fsHandle.go

Run: 
./loopback [loopback-dir path] [mount-dir path]

This is based on bazil/fuse and is supported for ubuntu 18.04 LTS