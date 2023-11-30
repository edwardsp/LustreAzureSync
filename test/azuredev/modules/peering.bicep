targetScope = 'resourceGroup'

param name string
param vnetName string
param allowGateway bool = true
param vnetId string

resource peeredVirtualNetwork 'Microsoft.Network/virtualNetworks@2023-05-01' existing = { 
  name: vnetName
}

resource peering 'Microsoft.Network/virtualNetworks/virtualNetworkPeerings@2023-05-01' = {
  name: name
  parent: peeredVirtualNetwork
  properties: {
    allowVirtualNetworkAccess: true
    allowForwardedTraffic: true
    allowGatewayTransit: allowGateway
    remoteVirtualNetwork: {
      id: vnetId
    }
  }
}
