#!/bin/bash

apt update

# install azure cli
curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash

apt install -y git

# install vscode
curl -sSL https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://packages.microsoft.com/repos/vscode stable main"
apt update
apt install -y code

# install lustre
#apt install -y ca-certificates curl apt-transport-https lsb-release gnupg
#source /etc/lsb-release
#echo "deb [arch=amd64] https://packages.microsoft.com/repos/amlfs-${DISTRIB_CODENAME}/ ${DISTRIB_CODENAME} main" | tee /etc/apt/sources.list.d/amlfs.list
#curl -sL https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor | tee /etc/apt/trusted.gpg.d/microsoft.gpg > /dev/null
#apt update

# install lustre
apt update && apt install -y ca-certificates curl apt-transport-https lsb-release gnupg
source /etc/lsb-release
echo "deb [arch=amd64] https://packages.microsoft.com/repos/amlfs-${DISTRIB_CODENAME}-test/ ${DISTRIB_CODENAME} main" | tee /etc/apt/sources.list.d/amlfs.list
curl -sL https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor | tee /etc/apt/trusted.gpg.d/microsoft.gpg > /dev/null
apt update
apt install -y amlfs-lustre-client-2.15.3-43-gd7e07df=$(uname -r)

# install go
cd /opt && wget -qO- https://go.dev/dl/go1.21.4.linux-amd64.tar.gz | tar zxvf -
apt install -y bzip2 g++ g++-11 gcc gcc-11 libasan6 libatomic1 libcc1-0 libdpkg-perl libfile-fcntllock-perl