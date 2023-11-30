package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/directory"
)

func main() {
	// Define command-line arguments
	flag.Parse()

	// Check if the required command-line arguments are provided
	if flag.NArg() != 4 {
		fmt.Println("Usage: go run main.go <accountName> <containerName> <originalPath> <newName>")
		return
	}

	// Read the command-line arguments
	accountName := flag.Arg(0)
	containerName := flag.Arg(1)
	originalPath := flag.Arg(2)
	newName := flag.Arg(3)

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		slog.Error("Unable to create default azure credential", "err", err)
		os.Exit(1)
	}

	path := fmt.Sprintf("https://%s.dfs.core.windows.net/%s/%s", accountName, containerName, originalPath)

	client, err := directory.NewClient(path, cred, nil)
	if err != nil {
		slog.Error("Unable to create directory client", "err", err)
		os.Exit(1)
	}

	_, err = client.Create(context.Background(), nil)
	if err != nil {
		slog.Error("Unable to get directory", "err", err)
		os.Exit(1)
	}

	_, err = client.Rename(context.Background(), newName, nil)
	if err != nil {
		slog.Error("Unable to rename directory", "err", err)
		os.Exit(1)
	}

	fmt.Println("File or directory moved successfully!")
}
