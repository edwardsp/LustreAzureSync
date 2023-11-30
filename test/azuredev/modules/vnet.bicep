param location string = resourceGroup().location
param vnetName string
param cidr string
var subnetName = 'default'
param peeredVnetName string
param peeredResourceGroupName string

resource nsg 'Microsoft.Network/networkSecurityGroups@2022-07-01' = {
  name: '${vnetName}-nsg'
  location: location
  properties: {
    securityRules: [
      {
        name: 'AllowVnetToVnetSsh'
        properties: {
          protocol: '*'
          sourcePortRange: '*'
          destinationPortRange: '22'
          sourceAddressPrefix: 'VirtualNetwork'
          destinationAddressPrefix: 'VirtualNetwork'
          access: 'Allow'
          priority: 100
          direction: 'Inbound'
        }
      }      
    ]
  }
}

resource vnet 'Microsoft.Network/virtualNetworks@2023-05-01' = {
  name: vnetName
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: [ cidr ]
    }
    subnets: [{
      name: subnetName
      properties: {
        addressPrefix: cidr
        networkSecurityGroup: {
          id: nsg.id
        }
      }
    }]
  }
}

resource vnetPeering 'Microsoft.Network/virtualNetworks/virtualNetworkPeerings@2023-05-01' = {
  name: '${peeredResourceGroupName}-${peeredVnetName}'
  parent: vnet
  properties: {
    allowVirtualNetworkAccess: true
    allowForwardedTraffic: true
    useRemoteGateways: true
    remoteVirtualNetwork: {
      id: resourceId(peeredResourceGroupName, 'Microsoft.Network/virtualNetworks', peeredVnetName)
    }
  }
}

output vnetId string = vnet.id
output subnetId string = '${vnet.id}/subnets/${subnetName}'
