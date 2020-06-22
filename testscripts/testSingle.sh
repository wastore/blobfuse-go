echo "TESTING SINGLE FILE AT A TIME"
mkdir mount temp anuj
export AZURE_STORAGE_ACCOUNT=anmodblobwthns
export AZURE_STORAGE_ACCESS_KEY=vddsBzGQzl/pd0WLsbKGSYye+RfgpS93fiIoXoSBzRYjPDK+dlVTOZJqJ/G0A+u/VnEoDXQS5Yjx+jazCC4BfA==
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:/home/t-anmodi/go/bin

echo "Upload Test"
echo "Blobfuse-CPP"

for ((n=0;n<3;n++))
do
    cd ~
    blobfuse mount --tmp-path=temp --container-name=test
    cd mount
    STARTTIME=$(date +%s)
    dd if=/dev/zero of=test1.txt  bs=10M  count=1
    ENDTIME=$(date +%s)
    echo "$(($ENDTIME - $STARTTIME)) seconds to upload 10 MB"
    STARTTIME=$(date +%s)
    dd if=/dev/zero of=test2.txt  bs=20M  count=1
    ENDTIME=$(date +%s)
    echo "$(($ENDTIME - $STARTTIME)) seconds to upload 20 MB"
    STARTTIME=$(date +%s)
    dd if=/dev/zero of=test3.txt  bs=30M  count=1
    ENDTIME=$(date +%s)
    echo "$(($ENDTIME - $STARTTIME)) seconds to upload 30 MB"
    rm test1.txt test2.txt test3.txt
    cd ..
    fusermount -u mount
done

echo "Blobfuse-Go"
for ((n=0;n<3;n++))
do
    cd ~/Desktop/msft/blobfuse-go/main
    ./filesystem --mountPath=~/mount --accountName=anmodblobwthns --accountKey=vddsBzGQzl/pd0WLsbKGSYye+RfgpS93fiIoXoSBzRYjPDK+dlVTOZJqJ/G0A+u/VnEoDXQS5Yjx+jazCC4BfA== --containerName=test
    cd ~/mount
    STARTTIME=$(date +%s)
    dd if=/dev/zero of=test1.txt  bs=10M  count=1
    ENDTIME=$(date +%s)
    echo "$(($ENDTIME - $STARTTIME)) seconds to upload 10 MB"
    STARTTIME=$(date +%s)
    dd if=/dev/zero of=test2.txt  bs=20M  count=1
    ENDTIME=$(date +%s)
    echo "$(($ENDTIME - $STARTTIME)) seconds to upload 20 MB"
    STARTTIME=$(date +%s)
    dd if=/dev/zero of=test3.txt  bs=30M  count=1
    ENDTIME=$(date +%s)
    echo "$(($ENDTIME - $STARTTIME)) seconds to upload 30 MB"
    cd ..
    fusermount -u mount
    blobfuse ~/mount --tmp-path=~/temp --container-name=test
    cd mount
    rm test1.txt test2.txt test3.txt
    cd ..
    fusermount -u mount
done

echo "AZCOPY"

