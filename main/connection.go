package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

var (
	serviceURL   azblob.ServiceURL
	ctx          context.Context
	containerURL azblob.ContainerURL
)

// ValidateAccount verifies storage account credentials and returns a connection
func ValidateAccount() (errno int) {

	credential, err := azblob.NewSharedKeyCredential(AccountName, AccountKey)
	if err != nil {
		log.Printf("%v", err)
		log.Printf("Error in NewShared KEy")
		return 1
	}
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", AccountName))
	serviceURL = azblob.NewServiceURL(*u, p)

	// Try to list the blobs to verify the connection and account
	ctx = context.Background()
	containerURL = serviceURL.NewContainerURL(ContainerName)
	marker := (azblob.Marker{})
	_, err = containerURL.ListBlobsHierarchySegment(ctx, marker, "/", azblob.ListBlobsSegmentOptions{})
	if err != nil {
		log.Printf("List me Error h")
		log.Fatal(err)
		return 1
	}
	return 0
}

// GetBlobItems return list of blobs in the storage account
func GetBlobItems(prefix string) (blobItems []azblob.BlobItem) {
	// log.Printf("Get Blob Items: %s", prefix)
	for marker := (azblob.Marker{}); marker.NotDone(); {
		// Get a result segment starting with the blob indicated by the current Marker.
		options := azblob.ListBlobsSegmentOptions{}
		options.Details.Metadata = true
		if prefix != "" {
			options.Prefix = prefix
		}
		listBlob, err := containerURL.ListBlobsHierarchySegment(ctx, marker, "/", options)
		if err != nil {
			fmt.Printf("Error")
			log.Fatal(err)
		}
		// IMPORTANT: ListBlobs returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = listBlob.NextMarker

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlob.Segment.BlobItems {
			namearray := strings.Split(blobInfo.Name, "/")
			blobInfo.Name = namearray[len(namearray)-1]
			// log.Printf(blobInfo.Name)
			blobItems = append(blobItems, blobInfo)
		}
	}
	return blobItems
}

// ReadBlobContents returns the byte array of the content of blob
func ReadBlobContents(blobName string, blobsize uint64) []byte {
	// log.Printf("RedBlobContent: %s", blobName)
	blobURL := containerURL.NewBlobURL(blobName)
	b := make([]byte, blobsize)
	o := azblob.DownloadFromBlobOptions{
		Parallelism: 5,
	}
	err := azblob.DownloadBlobToBuffer(ctx, blobURL, 0, 0, b, o)
	if err != nil {
		log.Fatal(err)
	}
	return b
}

// UploadBlobContents returns status
func UploadBlobContents(blobName string, data []byte, isDir bool) int {
	// log.Printf("UploadBlobContent: %s", blobName)
	blobURL := containerURL.NewBlockBlobURL(blobName)
	metadata := azblob.Metadata{}
	if isDir {
		metadata = azblob.Metadata{
			"hdi_isFolder": "true",
		}
	}
	o := azblob.UploadToBlockBlobOptions{
		Metadata:    metadata,
		Parallelism: 5,
	}
	_, err := azblob.UploadBufferToBlockBlob(ctx, data, blobURL, o)
	if err != nil {
		return 1
	}
	return 0
}
