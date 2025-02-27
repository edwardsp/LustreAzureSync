package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"sync"

	//"log"
	"log/slog"

	//"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/file"

	"github.com/edwardsp/LustreAzureSync/src/go-lustre/llapi"
	//_ "net/http/pprof"
)

var mountRoot string
var rootFid string
var archiveId uint

var containerName string
var ctx context.Context
var cred *azidentity.DefaultAzureCredential
var client *azblob.Client
var serviceUrl string
var usingHns bool
var autoRemove bool
var maxConcurrency int
var maxRetries int

var version = "dev"

type lfsent struct {
	name   string
	parent string
}

var dirLookup map[string]lfsent
var symlinkLookup map[string]lfsent

func getPath(name string, fid string) (string, error) {
	path := name

	e, ok := dirLookup[fid]
	if !ok {
		msg := fmt.Sprintf("failed to find fid %s [ name='%s' ]", fid, name)
		return "", errors.New(msg)
	}
	for f := fid; f != rootFid; f = e.parent {
		e, ok = dirLookup[f]
		if !ok {
			msg := fmt.Sprintf("failed to find fid %s [ name='%s', path='%s' ]", fid, name, path)
			return "", errors.New(msg)
		}
		path = e.name + "/" + path
	}

	return path, nil
}

func get_meta(fname string) (_ map[string]*string, err error) {
	meta := make(map[string]*string)
	fileName := path.Join(mountRoot, fname)
	fileInfo, err := os.Lstat(fileName)
	if err != nil {
		return nil, err
	}

	isDir := fileInfo.IsDir()
	isSymlink := fileInfo.Mode()&os.ModeSymlink != 0

	if isDir {
		meta["hdi_isfolder"] = to.Ptr("true")
	}

	meta["modtime"] = to.Ptr(fileInfo.ModTime().Format("2006-01-02 15:04:05 -0700"))
	meta["owner"] = to.Ptr(fmt.Sprintf("%d", fileInfo.Sys().(*syscall.Stat_t).Uid))
	meta["group"] = to.Ptr(fmt.Sprintf("%d", fileInfo.Sys().(*syscall.Stat_t).Gid))

	// only add permissions if not a symlink
	if !isSymlink {
		permissions := uint32(fileInfo.Mode().Perm())
		if fileInfo.Mode()&os.ModeSticky != 0 {
			permissions |= syscall.S_ISVTX
		}
		meta["permissions"] = to.Ptr(fmt.Sprintf("%04o", permissions))
	}

	// add symlink target
	if isSymlink {
		link_target, err := os.Readlink(fileName)
		if err != nil {
			fmt.Printf("error: failed to read the symlink target for %s", fname)
		}
		meta["symlink"] = to.Ptr(link_target)
		meta["ftype"] = to.Ptr("LNK")
	}

	return meta, nil
}

// Retry function with backoff
func retry(fn func() error) error {
	for i := 0; i < maxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		if i == maxRetries-1 {
			return err
		}
		backoff := time.Duration(i+1) * time.Second
		slog.Debug("Retrying after error", "error", err, "backoff", backoff)
		time.Sleep(backoff)
	}
	return nil
}

// Function to delete a blob
func delete_blob(path string) {
	if _, err := os.Stat(mountRoot + "/" + path); os.IsNotExist(err) {
		err := retry(func() error {
			_, err := client.DeleteBlob(ctx, containerName, path, nil)
			return err
		})
		if err != nil {
			slog.Warn("Failed to delete object", "path", path, "error", err)
		}
	}
}

func create_symlink(name string) {
	meta, err := get_meta(name)
	if err != nil {
		slog.Warn("Failed to get metadata for slink", "name", name)
	} else {
		// only create in blob storage if it is a symlink on the filesystem
		if _, ok := meta["symlink"]; ok {
			err := retry(func() error {
				_, err := client.UploadBuffer(ctx, containerName, name, []byte(*meta["symlink"]), &azblob.UploadBufferOptions{
					Metadata: meta,
				})
				return err
			})
			if err != nil {
				slog.Warn("Failed to create symlink", "name", name, "error", err)
			}
		} else {
			slog.Warn("Not a symlink on the filesystem anymore, not creating", "name", name)
		}
	}
}

func set_metadata(name string) {
	meta, err := get_meta(name)
	if err != nil {
		slog.Warn("Failed to get metadata for file", "name", name, "error", err)
		return
	} else {
		if usingHns == true {
			// delete hdi_isfolder from meta if it exists
			if _, ok := meta["hdi_isfolder"]; ok {
				delete(meta, "hdi_isfolder")
			}
		}

		blobUrl := fmt.Sprintf("%s/%s/%s", serviceUrl, containerName, name)
		blobClient, err := blob.NewClient(blobUrl, cred, nil)
		if err != nil {
			slog.Warn("Failed to get blobClient", "name", name, "error", err)
		}
		_, err = blobClient.SetMetadata(ctx, meta, nil)
		if err != nil {
			slog.Warn("Failed to set metadata", "name", name, "error", err)
		}
	}
}

func slink(rec *llapi.ChangelogRecord) {
	tname, err := getPath(rec.Name(), rec.ParentFid().String())
	if err != nil {
		slog.Warn("slink: Failed to get path", "name", rec.Name(), "error", err)
		return
	}
	symlinkLookup[rec.TargetFid().String()] = lfsent{rec.Name(), rec.ParentFid().String()}

	slog.Info("slink", "idx", rec.Index(), "type", rec.Type(), "targetfid", rec.TargetFid(), "tname", tname, "rec.SourceName()", rec.SourceName(), "rec.Name()", rec.Name(), "rec.JobID", rec.JobID())
	create_symlink(tname)
}

// only handle symlinks here
func unlnk(rec *llapi.ChangelogRecord) {
	// if the target is in symlinkLookup we need to delete it
	if _, ok := symlinkLookup[rec.TargetFid().String()]; ok {
		tname, err := getPath(rec.Name(), rec.ParentFid().String())
		if err != nil {
			slog.Warn("unlnk: Failed to get path", "name", rec.Name(), "error", err)
			return
		}

		if tname == "" {
			slog.Warn("Cannot unlink", "name", rec.Name())
			return
		}

		delete(symlinkLookup, rec.TargetFid().String())

		slog.Info("unlnk symlink", "idx", rec.TargetFid(), "tname", tname)
		delete_blob(tname)
	} else {
		if autoRemove == true {
			tname, err := getPath(rec.Name(), rec.ParentFid().String())
			if err != nil {
				slog.Warn("unlnk: Failed to get path", "name", rec.Name(), "error", err)
				return
			}

			if tname == "" {
				slog.Warn("Cannot unlink", "name", rec.Name())
				return
			}

			slog.Info("unlnk file", "targetfid", rec.TargetFid(), "tname", tname)
			delete_blob(tname)
		}
	}
}

func create_dir(name string) {
	meta, err := get_meta(name)
	if err != nil {
		slog.Warn("Failed to get metadata for directory", "name", name)
	} else {
		// only create in blob storage if it is a directory on the filesystem
		if _, ok := meta["hdi_isfolder"]; !ok {
			slog.Warn("Not a directory on the filesystem anymore, not creating", "name", name)
		} else {
			err := retry(func() error {
				_, err := client.UploadBuffer(ctx, containerName, name, nil, &azblob.UploadBufferOptions{
					Metadata: meta,
				})
				return err
			})
			if err != nil {
				slog.Warn("Failed to create directory", "name", name, "error", err)
			}
		}
	}
}

func mkdir(rec *llapi.ChangelogRecord) {
	//var recno int64 = 0
	//var linkno int = 0
	//tname, _ := llapi.Fid2Path(mdtname, rec.TargetFid(), &recno, &linkno)
	tname, err := getPath(rec.Name(), rec.ParentFid().String())
	if err != nil {
		slog.Warn("mkdir: Failed to get path", "name", rec.Name(), "error", err)
		return
	}

	if tname == "" {
		slog.Warn("Cannot mkdir", "name", rec.Name())
		return
	}

	dirLookup[rec.TargetFid().String()] = lfsent{rec.Name(), rec.ParentFid().String()}

	slog.Info("mkdir", "idx", rec.Index(), "type", rec.Type(), "targetfid", rec.TargetFid(), "tname", tname, "rec.SourceName()", rec.SourceName(), "rec.Name()", rec.Name(), "rec.JobID", rec.JobID())
	create_dir(tname)
}

func rmdir(rec *llapi.ChangelogRecord) {
	tname, err := getPath(rec.Name(), rec.ParentFid().String())
	if err != nil {
		slog.Warn("rmdir: Failed to get target path", "name", rec.Name(), "error", err)
		return
	}

	if tname == "" {
		slog.Warn("Cannot rmdir", "name", rec.Name())
		return
	}

	delete(dirLookup, rec.TargetFid().String())
	slog.Info("rmdir", "idx", rec.Index(), "type", rec.Type(), "targetfid", rec.TargetFid(), "tname", tname, "rec.SourceName()", rec.SourceName(), "rec.Name()", rec.Name(), "rec.JobID", rec.JobID())
	delete_blob(tname)
}

func update_dirLookup(rec *llapi.ChangelogRecord) {
	if _, ok := dirLookup[rec.SourceFid().String()]; ok {
		sfid := rec.SourceFid()
		ent := dirLookup[sfid.String()]
		ent.name = rec.Name()
		ent.parent = rec.ParentFid().String()
		dirLookup[sfid.String()] = ent

		//fmt.Printf("sfid=%s name=%s parent=%s\n", sfid, dirLookup[sfid.String()], dirLookup[dirLookup[sfid.String()].parent])
	}
}

func update_symlinkLookup(rec *llapi.ChangelogRecord) {
	// if sfid is in symlinkLookup change the name (i.e. it is symlink)
	if _, ok := symlinkLookup[rec.SourceFid().String()]; ok {
		sfid := rec.SourceFid()
		ent := symlinkLookup[sfid.String()]
		ent.name = rec.Name()
		ent.parent = rec.ParentFid().String()
		symlinkLookup[sfid.String()] = ent
		//fmt.Printf("sfid=%s name=%s parent=%s\n", sfid, dirLookup[sfid.String()], dirLookup[dirLookup[sfid.String()].parent])
	}
}

func renme_adls(rec *llapi.ChangelogRecord) error {
	tname, err := getPath(rec.Name(), rec.ParentFid().String())
	if err != nil {
		return err
	}
	sname, err := getPath(rec.SourceName(), rec.SourceParentFid().String())
	if err != nil {
		return err
	}
	if tname == sname {
		return errors.New("Source and target have the same name")
	}

	_, isDir := dirLookup[rec.SourceFid().String()]
	path := fmt.Sprintf("%s/%s/%s", serviceUrl, containerName, sname)
	if isDir {
		client, err := directory.NewClient(path, cred, nil)
		if err != nil {
			return errors.New("Unable to create directory client")
		}
		_, err = client.Rename(ctx, tname, nil)
		if err != nil {
			return errors.New("Unable to rename directory")
		}

	} else {
		client, err := file.NewClient(path, cred, nil)
		if err != nil {
			return errors.New("Unable to create file client")
		}
		_, err = client.Rename(ctx, tname, nil)
		if err != nil {
			return errors.New("Unable to rename file")
		}
	}

	update_dirLookup(rec)
	update_symlinkLookup(rec)

	return nil
}

func move_blob(sourcePath string, targetPath string, checkSourceExists bool) {
	targetBlobUrl := fmt.Sprintf("%s/%s/%s", serviceUrl, containerName, targetPath)
	targetBlobClient, err := blob.NewClient(targetBlobUrl, cred, nil)
	if err != nil {
		slog.Warn("Failed to get targetBlobClient", "targetBlobUrl", targetBlobUrl, "error", err)
	}

	sourceBlobUrl := fmt.Sprintf("%s/%s/%s", serviceUrl, containerName, sourcePath)
	sourceBlobClient, err := blob.NewClient(sourceBlobUrl, cred, nil)
	if err != nil {
		slog.Warn("Failed to get sourceBlobClient", "sourceBlobUrl", sourceBlobUrl, "error", err)
	}

	if checkSourceExists {
		_, err := sourceBlobClient.GetProperties(ctx, &blob.GetPropertiesOptions{})
		if err != nil {
			slog.Debug("Source blob does not exist", "sourceBlobUrl", sourceBlobUrl, "error", err)
			return
		}

	}

	// copy in blob (async)
	retry(func() error {
		_, err = targetBlobClient.StartCopyFromURL(ctx, sourceBlobClient.URL(), nil)
		return err
	})
	if err != nil {
		slog.Warn("Failed to copy blob", "sourceBlobUrl", sourceBlobUrl, "targetBlobUrl", targetBlobUrl, "error", err)
		return
	}
	// wait for copy to complete
	for {
		props, err := targetBlobClient.GetProperties(ctx, &blob.GetPropertiesOptions{})
		if err != nil {
			slog.Warn("Failed to get blob properties", "targetBlobUrl", targetBlobUrl, "error", err)
			return
		}
		if props.CopyStatus == nil {
			slog.Warn("CopyStatus is nil on the blob properties, retrying after 1 second", "targetBlobUrl", targetBlobUrl)
			time.Sleep(time.Second)
			continue
		}
		if *props.CopyStatus == blob.CopyStatusTypePending {
			time.Sleep(time.Second)
			continue
		}
		if *props.CopyStatus == blob.CopyStatusTypeSuccess {
			break
		}
		if *props.CopyStatus == blob.CopyStatusTypeFailed {
			slog.Warn("Copy failed", "targetBlobUrl", targetBlobUrl)
			return
		}
		if *props.CopyStatus == blob.CopyStatusTypeAborted {
			slog.Warn("Copy aborted", "targetBlobUrl", targetBlobUrl)
			return
		}
	}

	delete_blob(sourcePath)
}

func renme_copyblob(rec *llapi.ChangelogRecord) {
	tname, err := getPath(rec.Name(), rec.ParentFid().String())
	if err != nil {
		slog.Warn("renme_copyblob: Failed to get target path", "name", rec.Name(), "source", rec.SourceName(), "error", err)
		return
	}
	sname, err := getPath(rec.SourceName(), rec.SourceParentFid().String())
	if err != nil {
		slog.Warn("renme_copyblob: Failed to get source path", "name", rec.Name(), "source", rec.SourceName(), "error", err)
		return
	}
	slog.Info("Renaming", "idx", rec.Index(), "source", sname, "target", tname)
	if tname == sname {
		slog.Warn("Source and target have the same name", "source", sname, "target", tname)
		return
	}

	_, isDir := dirLookup[rec.SourceFid().String()]

	if !isDir {
		move_blob(sname, tname, true)
	} else {
		semaphore := make(chan struct{}, maxConcurrency)
		var wg sync.WaitGroup

		slog.Info("Moving archived files in BLOB storage")
		move_blob(sname, tname, false)
		dirprefix := sname + "/"
		counter := 0
		pager := client.NewListBlobsFlatPager(containerName, &azblob.ListBlobsFlatOptions{
			Include: container.ListBlobsInclude{Deleted: false, Metadata: true, Versions: false},
			Prefix:  &dirprefix,
		})
		for pager.More() {
			resp, err := pager.NextPage(ctx)
			if err != nil {
				slog.Warn("Error getting next page", "error", err)
				break
			}
			for _, blobItem := range resp.Segment.BlobItems {
				counter++
				if counter%10000 == 0 {
					slog.Info("Moving archived files in BLOB storage", "count", counter)
				}

				sourcePath := *blobItem.Name
				targetPath := strings.Replace(sourcePath, sname, tname, 1)
				slog.Info("Moving archived file in BLOB storage", "sourcePath", sourcePath, "targetPath", targetPath)

				wg.Add(1)
				semaphore <- struct{}{}

				go func() {
					defer wg.Done()
					defer func() { <-semaphore }()
					move_blob(sourcePath, targetPath, false)
				}() // go func
			}
		}
		slog.Info("Moved all archived files from BLOB storage", "count", counter)
		wg.Wait()
		close(semaphore)
	}

	slog.Info("Move complete")

	update_dirLookup(rec)
	update_symlinkLookup(rec)

}

// we only need to update the layout for a directory
func update_layout(rec *llapi.ChangelogRecord) {
	tfid := rec.TargetFid()

	recno := int64(0)
	linkno := 0
	target_name, err := llapi.Fid2Path(mountRoot, tfid, &recno, &linkno)
	if err != nil {
		slog.Warn("update_layout: Failed to get path", "name", rec.Name(), "error", err)
		return
	}

	if target_name == "" {
		slog.Info("Cannot update Lustre mount metadata")
		return
	}

	if _, ok := dirLookup[tfid.String()]; ok {
		set_metadata(target_name)
	}
}

func update_metadata(rec *llapi.ChangelogRecord) {
	tfid := rec.TargetFid()

	recno := int64(0)
	linkno := 0
	target_name, err := llapi.Fid2Path(mountRoot, tfid, &recno, &linkno)
	if err != nil {
		if rec.Name() == "" {
			// this can happen when the file is deleted before we get to it
			return
		}
		slog.Warn("update_metadata: Failed to get path", "name", rec.Name(), "error", err)
		return
	}

	if target_name == "" {
		slog.Info("Cannot update Lustre mount metadata")
		return
	}

	// if it is a directory, just update, don't try and get hsm status
	if _, ok := dirLookup[tfid.String()]; ok {
		set_metadata(target_name)
		return
	}

	// if it is a symlink, just update, don't try and get hsm status
	if _, ok := symlinkLookup[tfid.String()]; ok {
		set_metadata(target_name)
		return
	}

	// rec is a file, so we need to get the hsm status and only update if not dirty anyway
	state, _, err := llapi.GetHsmFileStatus(mountRoot + "/" + target_name)
	if err != nil {
		slog.Warn("Failed to get HSM file status", "path", target_name, "error", err)
		return
	}
	// if the file exists and is not dirty, update the metadata
	if state.HasFlag(llapi.HsmFileExists) && !state.HasFlag(llapi.HsmFileDirty) {
		set_metadata(target_name)
	}
}

// walk the filesystem and put all paths into a map
// NOTE: all errors are written to the error log but the walk will continue
func walk_filesystem(root string) (dirFidToPath map[string]lfsent, symlinkFidToPath map[string]lfsent) {
	dirFidToPath = make(map[string]lfsent)
	symlinkFidToPath = make(map[string]lfsent)
	nEntries := 0
	nSymlinks := 0
	nDirectories := 0
	filepath.Walk(root, func(path string, fileInfo os.FileInfo, err error) error {
		// Log errors and continue rather than stopping entirely.
		if err != nil {
			slog.Error("Error accessing path", "path", path, "error", err)
			return nil
		}
		// Additional check to ensure fileInfo is non-nil
		if fileInfo == nil {
			slog.Error("Nil fileInfo encountered", "path", path)
			return nil
		}
		nEntries++
		if nEntries%10000 == 0 {
			slog.Info("Walking filesystem", "total_entries", nEntries, "total_dirs", nDirectories, "total_symlinks", nSymlinks)
		}
		isDir := fileInfo.IsDir()
		isSymlink := fileInfo.Mode()&os.ModeSymlink != 0
		if !(isDir || isSymlink) {
			return nil
		}
		name := filepath.Base(path)
		parent := filepath.Dir(path)
		var fid string
		var pfid string = ""
		f, err := llapi.Path2Fid(path)
		if err == nil {
			fid = f.String()
		} else {
			slog.Error("failed to get fid", "path", path, "error", err)
			return nil // Continue walking rather than exiting
		}
		if path != root {
			f, err := llapi.Path2Fid(parent)
			if err == nil {
				pfid = f.String()
			} else {
				slog.Error("failed to get fid", "parent", parent, "error", err)
				return nil // Continue walking rather than exiting
			}
		}
		//fmt.Printf("%s : %s %s [%s]\n", fid, name, parent, path)
		if isDir {
			nDirectories++
			dirFidToPath[fid] = lfsent{name, pfid}
		} else {
			nSymlinks++
			symlinkFidToPath[fid] = lfsent{name, pfid}
		}

		return nil
	})
	return dirFidToPath, symlinkFidToPath
}

func pretty_print_changelog_record(rec *llapi.ChangelogRecord) {
	fmt.Println("Changelog record: " + rec.Type())
	fmt.Printf("  Index=%d\n", rec.Index())
	//fmt.Println("  IsLastRename=("+string(rec.IsLastRename()))
	//fmt.Println("  IsLastUnlink="+rec.IsLastUnlink())
	//fmt.Println("  IsRename="+rec.IsRename())
	fmt.Println("  JobID=" + rec.JobID())
	fmt.Println("  Name=" + rec.Name())
	fmt.Println("  ParentFid=" + rec.ParentFid().String())
	fmt.Println("  SourceFid=" + rec.SourceFid().String())
	fmt.Println("  SourceName=" + rec.SourceName())
	fmt.Println("  SourceParentFid=" + rec.SourceParentFid().String())
	fmt.Println("  String=" + rec.String())
	fmt.Println("  TargetFid=" + rec.TargetFid().String())
	fmt.Println("  Time=" + rec.Time().String())
	fmt.Println("  Type=" + rec.Type())
	fmt.Println("  TypeCode=" + string(rec.TypeCode()))
}

func process_changelog(mdtname string, userid string) {
	follow := true
	cl, err := llapi.ChangelogStart(mdtname, 0, follow)
	if err != nil {
		slog.Error("failed to start changelog", "error", err)
		return
	}

	var lastidx int64 = 0

	for {
		rec, err := llapi.ChangelogRecv(cl)

		if err != nil {
			if err.Error() == "EOF" {
				break
			}

			slog.Error("failed to get changelog record", "error", err)
			slog.Info("Trying to reconnect to the changelog at last index", "lastidx", lastidx)
			cl, err = llapi.ChangelogStart(mdtname, lastidx, follow)
			if err != nil {
				slog.Error("failed to restart changelog", "error", err)
				return
			}
			rec, err = llapi.ChangelogRecv(cl)
			if err != nil {
				slog.Error("failed to get changelog record (again)", "error", err)
				return
			}
			slog.Info("Restarted changelog", "startingIndex", rec.Index())
			time.Sleep(time.Second * 10)
			continue
		}

		lastidx = rec.Index()
		rectypeid := rec.TypeCode()
		switch {
		case rectypeid == llapi.OpMkdir:
			slog.Debug("ChangelogEntry", "type", rec.Type(), "idx", rec.Index(), "type", rec.Type(), "Name", rec.Name(), "SourceName", rec.SourceName())
			mkdir(rec)
		case rectypeid == llapi.OpRmdir:
			slog.Debug("ChangelogEntry", "type", rec.Type(), "idx", rec.Index(), "type", rec.Type(), "Name", rec.Name(), "SourceName", rec.SourceName())
			rmdir(rec)
		case rectypeid == llapi.OpRename:
			slog.Debug("ChangelogEntry", "type", rec.Type(), "idx", rec.Index(), "type", rec.Type(), "Name", rec.Name(), "SourceName", rec.SourceName())
			if usingHns == true {
				err := renme_adls(rec)
				if err != nil {
					slog.Warn("Failed to rename with adls", "error", err)
					renme_copyblob(rec)
				}
			} else {
				renme_copyblob(rec)
			}
		case rectypeid == llapi.OpSetattr:
			slog.Debug("ChangelogEntry", "type", rec.Type(), "idx", rec.Index(), "type", rec.Type(), "Name", rec.Name(), "SourceName", rec.SourceName())
			update_metadata(rec)
		case rectypeid == llapi.OpSoftlink:
			slog.Debug("ChangelogEntry", "type", rec.Type(), "idx", rec.Index(), "type", rec.Type(), "Name", rec.Name(), "SourceName", rec.SourceName())
			slink(rec)
		case rectypeid == llapi.OpUnlink:
			slog.Debug("ChangelogEntry", "type", rec.Type(), "idx", rec.Index(), "type", rec.Type(), "Name", rec.Name(), "SourceName", rec.SourceName())
			unlnk(rec)
		default:
			slog.Debug("ChangelogEntry", "type", rec.Type(), "idx", rec.Index(), "type", rec.Type(), "Name", rec.Name(), "SourceName", rec.SourceName())
		}
		//slog.Info("Map sizes", "dirLookup", len(dirLookup), "symlinkLookup", len(symlinkLookup))

		if lastidx%1000 == 0 {
			fmt.Printf("Clearing changelog up to index %d\n", lastidx)
			llapi.ChangelogClear(mdtname, userid, lastidx)
		}
	}

	fmt.Printf("Last index = %d\n", lastidx)
}

func main() {
	//go func() {
	//	log.Println(http.ListenAndServe("localhost:6060", nil))
	//}()

	var accountName, accountSuffix string
	var mdtname, userid string
	var debug, showVersion bool

	flag.StringVar(&accountName, "account", "", "Azure storage account name [required]")
	flag.StringVar(&accountSuffix, "suffix", "blob.core.windows.net", "Azure storage account suffix")
	flag.StringVar(&containerName, "container", "", "Azure storage container name [required]")
	flag.StringVar(&mdtname, "mdt", "", "MDT name [required]")
	flag.StringVar(&userid, "userid", "", "The Lustre changlelog User ID [required]")
	flag.StringVar(&mountRoot, "mountroot", "/lustre", "The lustre mount root")
	flag.BoolVar(&usingHns, "hns", false, "Use hierarchical namespace")
	flag.UintVar(&archiveId, "archiveid", 1, "The archive ID to use")
	flag.BoolVar(&autoRemove, "autoremove", false, "Automatically remove files from archive")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")
	flag.IntVar(&maxConcurrency, "maxconcurrency", 16, "Maximum concurrency for blob operations")
	flag.IntVar(&maxRetries, "maxretries", 3, "Maximum retries for blob operations")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")

	flag.Parse()

	if debug {
		opts := &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
		logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
		slog.SetDefault(logger)
	}

	if maxRetries < 1 {
		slog.Info("Setting max retries to 1")
		maxRetries = 1
	}

	if showVersion {
		fmt.Printf("Version: %s\n", version)
		os.Exit(0)
	}

	if len(accountName) == 0 {
		slog.Error("missing required account argument")
		os.Exit(1)
	}
	if len(containerName) == 0 {
		slog.Error("missing required container argument")
		os.Exit(1)
	}
	if len(mdtname) == 0 {
		slog.Error("missing required mdt argument")
		os.Exit(1)
	}
	if len(userid) == 0 {
		slog.Error("missing required userid argument")
		os.Exit(1)
	}

	var err error
	cred, err = azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		slog.Error("unable to get credential")
		os.Exit(1)
	}
	serviceUrl = fmt.Sprintf("https://%s.%s", accountName, strings.TrimPrefix(accountSuffix, "dfs."))
	slog.Info("serviceUrl", "serviceUrl", serviceUrl)
	ctx = context.Background()
	client, err = azblob.NewClient(serviceUrl, cred, nil)
	if err != nil {
		slog.Error("unable to create new client")
		os.Exit(1)
	}

	// set the global rootFid
	rf, err := llapi.Path2Fid(mountRoot)
	if err != nil {
		slog.Error("Failed to get root fid", "mountRoot", mountRoot)
		os.Exit(1)
	}
	rootFid = rf.String()

	if usingHns {
		slog.Info("Feature enabled: Hierarchical Namespace")
	}
	if autoRemove {
		slog.Info("Feature enabled: Auto Remove")
	}

	slog.Info("Initialising")
	dirLookup, symlinkLookup = walk_filesystem(mountRoot)
	slog.Info("Ready - starting changelog processing")
	process_changelog(mdtname, userid)
}
