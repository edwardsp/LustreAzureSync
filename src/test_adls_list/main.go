package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/filesystem"
)

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
	if err != nil {
		slog.Error("Unable to create default azure credential", "err", err)
		os.Exit(1)
	}

	fsurl := fmt.Sprintf("https://%s.dfs.core.windows.net/%s", accountName, containerName)

	client, err := filesystem.NewClient(fsurl, cred, nil)
	if err != nil {
		slog.Error("Unable to create filesystem client", "err", err)
		os.Exit(1)
	}

	options := &filesystem.ListPathsOptions{}
	options.Prefix = &path

	pager := client.NewListPathsPager(true, options)
	if err != nil {
		slog.Error("Unable to get new list paths pager", "err", err)
		os.Exit(1)
	}

	for pager.More() {
		// advance to the next page
		page, err := pager.NextPage(context.TODO())
		if err != nil {
			slog.Error("Unable to get next page", "err", err)
			os.Exit(1)
		}

		// print the path names for this page
		for _, path := range page.PathList.Paths {
			fmt.Print(*path.Name)
			if path.IsDirectory != nil {
				fmt.Println(" (directory)")
			} else {
				fmt.Println(" (file)")
			}
		}
	}

	fmt.Println("List completed successfully!")
}
