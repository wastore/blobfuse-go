package connection

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// ValidateAccount verifies storage account credentials and returns a connection
func ValidateAccount(accountName string, accountKey string, container string) (errno int) {

	var serviceURL = azblob.ServiceURL{}
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Printf("%v", err)
		return 1
	}
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))
	serviceURL = azblob.NewServiceURL(*u, p)

	// Try to list the blobs to verify the connection and account
	ctx := context.Background()
	containerURL := serviceURL.NewContainerURL(container)

	for marker := (azblob.Marker{}); marker.NotDone(); {
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlob, err := containerURL.ListBlobsHierarchySegment(ctx, marker, "/", azblob.ListBlobsSegmentOptions{})
		if err != nil {
			log.Fatal(err)
			return 1
		}
		// IMPORTANT: ListBlobs returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = listBlob.NextMarker

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlob.Segment.BlobItems {
			fmt.Printf("Blob name: %v\n", blobInfo.Name)
		}
	}
	return 0
}
