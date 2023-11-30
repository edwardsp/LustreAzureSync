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

	// delete a blob with the specified path
	_, err = client.DeleteBlob(ctx, containerName, path, nil)
	handleError("failed to delete blob", err)

	fmt.Println("Delete completed successfully!")
}
