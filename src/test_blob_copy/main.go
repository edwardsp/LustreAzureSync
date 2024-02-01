package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
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
	if flag.NArg() != 4 {
		fmt.Println("Usage: go run main.go <accountName> <containerName> <source> <destination>")
		return
	}

	// Read the command-line arguments
	accountName := flag.Arg(0)
	containerName := flag.Arg(1)
	src := flag.Arg(2)
	dst := flag.Arg(3)

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	handleError("unable to get credential", err)

	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net", accountName)

	ctx := context.Background()

	client, err := azblob.NewClient(serviceURL, cred, nil)
	handleError("unable to create new client", err)

	containerClient := client.ServiceClient().NewContainerClient(containerName)

	srcBlob := containerClient.NewBlockBlobClient(src)
	dstBlob := containerClient.NewBlockBlobClient(dst)

	//_, err = dstBlob.CopyFromURL(ctx, srcBlob.URL(), nil)
	_, err = dstBlob.UploadBlobFromURL(ctx, srcBlob.URL(), nil)
	handleError("unable to copy blob", err)

	fmt.Println("Copy completed successfully!")
}
