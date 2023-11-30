targetScope = 'subscription'

param resourceGroupName string
param location string = deployment().location
param cidr string = '10.188.0.0/24'
param username string
@secure()
param publicKey string
param peeredVnetName string
param peeredResourceGroupName string

var vmName = 'devbox'
var vnetName = 'vnet'

resource rg 'Microsoft.Resources/resourceGroups@2022-09-01' = {
  name: resourceGroupName
  location: location
}


module vnet './modules/vnet.bicep' = {
  scope: rg
  name: 'vnet'
  params: {
    location: location
    vnetName: vnetName
    cidr: cidr
    peeredResourceGroupName: peeredResourceGroupName
    peeredVnetName: peeredVnetName
  }
}

module devVm './modules/devVm.bicep' = {
  scope: rg
  name: 'devVm'
  params: {
    location: location
    vmName: vmName
    username: username
    publicKey: publicKey
    subnetId: vnet.outputs.subnetId
  }
}

var storageAccountName = 'sa${uniqueString(subscription().subscriptionId, resourceGroupName)}x'
module storage './modules/storage.bicep' = {
  scope: rg
  name: 'storage'
  params: {
    location: location
    storageAccountName: storageAccountName
  }
}

module roleAssignmentRg './modules/roleAssignmentsRg.bicep' = {
  scope: rg
  name: 'roleAssignmentRg'
  params: {
    principalId: devVm.outputs.managedIdentity
  }
}

module vmInstall './modules/devVmInstall.bicep' = {
  scope: rg
  name: 'vmInstall'
  params: {
    location: location
    vmName: vmName
    logsUri: '${storage.outputs.blobEndpoint}logs'
  }
  dependsOn: [
    roleAssignmentRg
  ]
}

module peering './modules/peering.bicep' = {
  name: 'peerFrom${peeredVnetName}'
  scope: resourceGroup(peeredResourceGroupName)
  params: {
    name: '${resourceGroupName}_${vnetName}'
    vnetName: peeredVnetName
    allowGateway: true
    vnetId: vnet.outputs.vnetId
  }
}
