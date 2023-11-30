targetScope = 'resourceGroup'

param location string
param storageAccountName string

resource storageAccount 'Microsoft.Storage/storageAccounts@2023-01-01' = {
  name: storageAccountName
  location: location
  sku: {
    name: 'Standard_LRS'
  }
  kind: 'StorageV2'
  properties: {
    accessTier: 'Hot'
    minimumTlsVersion: 'TLS1_2'
  }
}

resource blobServices 'Microsoft.Storage/storageAccounts/blobServices@2023-01-01' = {
  name: 'default'
  parent: storageAccount
}

resource lustreArchive 'Microsoft.Storage/storageAccounts/blobServices/containers@2023-01-01' = {
  name: 'lustre'
  parent: blobServices
  properties: {
    publicAccess: 'None'
  }
}

resource logsArchive 'Microsoft.Storage/storageAccounts/blobServices/containers@2023-01-01' = {
  name: 'logs'
  parent: blobServices
  properties: {
    publicAccess: 'None'
  }
}

output blobEndpoint string = storageAccount.properties.primaryEndpoints.blob
