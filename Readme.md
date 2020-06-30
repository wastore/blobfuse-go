# blobfuse-go

This repo houses code for go implementation of blobfuse
Refer to the main package for the blobfuse-go code. For a filesystem example using bazil/fuse refer to loopback-example package

Based on file system library <a href="https://github.com/bazil/fuse">Bazil-fuse</a>
Uses Azure Storage Blob <a href="https://github.com/Azure/azure-storage-blob-go">SDK</a> for communicating with storage account container

<h2>Dependencies</h2>
Bazil Fuse Library: go get bazil.org/fuse
Azure SDK in Go: go get github.com/Azure/azure-storage-blob-go/azblob


<h2>Build Instruction:</h2>
Compile the file in the main package using following command: 
go build filesystem.go dirapis.go fileapis.go connection.go

This will create a executable named as filesystem

<h2>Run The File System Driver</h2>
To run executable along with following command line options
./filesystem --mountPath=/home/user/mountDir --accountName=nameOfStorageAccount --accountKey=accessKeyOfAccount --containerName=nameOfContainerToMount

This will start the file system application as a daemon. Now you can move inside the mounted directory to wrk with the azure stroage account container mounted.


<h2>Limitations and Future Work<h2>
Works for Ubuntu 18.04
Works for HNS disabled account
Authentication through Access Key only
Caching not implemented
