#!/bin/bash

# look at output as following:
# WORST CASE: 
#    grep $'\t'RELEASED$'\t'MISSING combined_status.txt
# NEEDS REARCHIVING: 
#    grep $'\t'ARCHIVED$'\t'MISSING combined_status.txt
# ONLY IN BLOB:
#    grep $'\t'MISSING$'\t' combined_status.txt
#
# Note: These results should be confirmed before taking any action as the hsm and blob status may have changed while the script was running.

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <lustre_root> <storage_account> <storage_container>"
    exit 1
fi

export LUSTRE_ROOT="$1"
export STORAGE_ACCOUNT="$2"
export STORAGE_CONTAINER="$3"

export CONTAINER_URL=https://${STORAGE_ACCOUNT}.blob.core.windows.net/${STORAGE_CONTAINER}
azcopy login --identity
echo "$(date +'%Y-%m-%d %H:%M:%S'): Getting the HSM status of the files"
find $LUSTRE_ROOT -type f -print0 \
    | xargs -r0 -L50 lfs hsm_state \
    | sed 's#^'$LUSTRE_ROOT'\/##g;s/:.*exists dirty.*$/\tDIRTY/g;s/:.*released exists.*$/\tRELEASED/g;s/:.*exists archived.*$/\tARCHIVED/g;s/: (0x00000000).*$/\tNONE/g' \
    > hsm_status.txt

# convert a stream of the following data format:
#  INFO: archived_file;  Content Length: 12
#  INFO: dirty_file;  Content Length: 38
#  INFO: released_file;  Content Length: 14
# the output should be filename,epoch_time,Content Length
echo "$(date +'%Y-%m-%d %H:%M:%S'): Getting the blob status of the files"
azcopy ls $CONTAINER_URL --machine-readable \
    | grep "Content Length" \
    | sed 's#INFO: \(.*\);  Content Length: \([0-9]*\)#\1\t\2#g' \
    > blob_status.txt

# now compare
echo "$(date +'%Y-%m-%d %H:%M:%S'): Sorting and joining the HSM and blob status"
sort -t $'\t' -k1,1 hsm_status.txt > hsm_status_sorted.txt
sort -t $'\t' -k1,1 blob_status.txt > blob_status_sorted.txt
join -t $'\t' -a1 -a2 -eMISSING -o 0,1.2,2.2 hsm_status_sorted.txt blob_status_sorted.txt > combined_status.txt

grep $'\t'RELEASED$'\t'MISSING combined_status.txt | cut -f1 > released_missing.txt
grep $'\t'ARCHIVED$'\t'MISSING combined_status.txt | cut -f1 > archived_missing.txt
grep $'\t'MISSING$'\t' combined_status.txt | cut -f1 > only_in_blob.txt
echo "$(date +'%Y-%m-%d %H:%M:%S'): Results are in released_missing.txt ($(wc -l <released_missing.txt)), archived_missing.txt ($(wc -l <archived_missing.txt)), and only_in_blob.txt ($(wc -l <only_in_blob.txt))"

echo "$(date +'%Y-%m-%d %H:%M:%S'): Done"