package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

func handleError(msg string, err error) {
	if err != nil {
		slog.Error(msg, "err", err)
		os.Exit(1)
	}
}

func main() {
	// Define command-line arguments
	flag.Parse()

	// Check if the required command-line arguments are provided
	if flag.NArg() != 3 {
		fmt.Println("Usage: go run main.go <accountName> <containerName> <path>")
		return
	}

	// Read the command-line arguments
	accountName := flag.Arg(0)
	containerName := flag.Arg(1)
	path := flag.Arg(2)

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	handleError("unable to get credential", err)

	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net", accountName)

	ctx := context.Background()

	client, err := azblob.NewClient(serviceURL, cred, nil)
	handleError("unable to create new client", err)

	pager := client.NewListBlobsFlatPager(containerName, &azblob.ListBlobsFlatOptions{
		Include: container.ListBlobsInclude{Deleted: false, Metadata: true, Versions: false},
		Prefix:  &path,
	})

	for pager.More() {
		resp, err := pager.NextPage(ctx)
		handleError("unable to read page", err) // if err is not nil, break the loop.
		for _, blob := range resp.Segment.BlobItems {
			fmt.Println(*blob.Name)
			// print all items of blob.MetaData map[string]*string
			for k, v := range blob.Metadata {
				fmt.Printf("  %s: %s\n", k, *v)
			}
		}
	}

	fmt.Println("List completed successfully!")
}
