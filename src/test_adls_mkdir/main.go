package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
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
		fmt.Println("Usage: go run main.go <accountName> <containerName> <localDir> <remoteDir>")
		return
	}
	// Read the command-line arguments
	accountName := flag.Arg(0)
	containerName := flag.Arg(1)
	localDir := flag.Arg(2)
	remoteDir := flag.Arg(3)

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	handleError("unable to get credential", err)

	fileInfo, err := os.Stat(localDir)
	handleError("unable to get file info", err)

	owner := fmt.Sprintf("%d", fileInfo.Sys().(*syscall.Stat_t).Uid)
	permissions := uint32(fileInfo.Mode().Perm())
	if fileInfo.Mode()&os.ModeSticky != 0 {
		permissions |= syscall.S_ISVTX
	}
	group := fmt.Sprintf("%d", fileInfo.Sys().(*syscall.Stat_t).Gid)
	modTime := fileInfo.ModTime().Format("2006-01-02 15:04:05 -0700")

	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net", accountName)
	ctx := context.Background()

	client, err := azblob.NewClient(serviceURL, cred, nil)
	handleError("unable to create new client", err)

	_, err = client.UploadBuffer(ctx, containerName, remoteDir, nil, &azblob.UploadBufferOptions{
		Metadata: map[string]*string{
			"hdi_isfolder": to.Ptr("true"),
			"permissions":  to.Ptr(fmt.Sprintf("%04o", permissions)),
			"modtime":      &modTime,
			"owner":        &owner,
			"group":        &group,
		},
	})
	handleError("unable to upload buffer", err)

	fmt.Println("Directory created successfully!")
}
