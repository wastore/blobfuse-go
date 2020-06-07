# blobfuse-go

This repo houses code for go implementation of blobfuse

Based on file system library <a href="https://github.com/bazil/fuse">Bazil-fuse</a>

Install bazil/fuse using:
go get bazil.org/fuse

File system for mounting azure storage account container as a virtual file system. 

Build Instruction:
Clone the repo and move inside main directory
go build filesystem.go

Run:
./filesystem [mount-dir path]