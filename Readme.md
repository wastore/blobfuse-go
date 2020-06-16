# blobfuse-go

This repo houses code for go implementation of blobfuse
Refer to the main package for the blobfuse-go code. For a filesystem example using bazil/fuse refer to loopback-example package

Based on file system library <a href="https://github.com/bazil/fuse">Bazil-fuse</a>

Install bazil/fuse using:
go get bazil.org/fuse

File system for mounting azure storage account container as a virtual file system. 


Build Instruction:
1. Clone the repo and move inside blobfuse-go
2. Create a folder credentials
3. Inside credentials folder create a file credentials.go
4. Add following content to this file:
        package credentials

        const (
            // AccountName holds dtorage account name
            AccountName = // Add the storae account name as string
            // AccountKey hold shared access key for account
            AccountKey = // Add Shared Access Key as string
            // ContainerName holds name of storage container in the acoount
            ContainerName = // Add Container name
        )
5. Move inside main directory Run this command to build: go build filesystem.go
6. Run the executable as: ./filesystem [mount-dir path]
