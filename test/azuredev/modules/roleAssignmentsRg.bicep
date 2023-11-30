targetScope = 'resourceGroup'

param principalId string

var rgContributorId = resourceId('microsoft.authorization/roleDefinitions', 'b24988ac-6180-42a0-ab88-20f7382dd24c')
var rgUserAccessAdministratorId = resourceId('microsoft.authorization/roleDefinitions', '18d7d88d-d35e-4fb5-a5c3-7773c20a72d9')
var rgStorageBlobDataContributorId = resourceId('microsoft.authorization/roleDefinitions', 'ba92f5b4-2d11-453d-a403-e96b0029c9fe')

resource rgContributorRa 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(principalId, rgContributorId)
  properties: {
    roleDefinitionId: rgContributorId
    principalId: principalId
    principalType: 'ServicePrincipal'
  }
}
resource rgUserAccessAdministratorRa 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(principalId, rgUserAccessAdministratorId)
  properties: {
    roleDefinitionId: rgUserAccessAdministratorId
    principalId: principalId
    principalType: 'ServicePrincipal'
  }
}
resource rgrgStorageBlobDataContributorRa 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(principalId, rgStorageBlobDataContributorId)
  properties: {
    roleDefinitionId: rgStorageBlobDataContributorId
    principalId: principalId
    principalType: 'ServicePrincipal'
  }
}


