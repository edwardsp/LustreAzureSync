targetScope = 'resourceGroup'

param location string = resourceGroup().location
param vmName string
param logsUri string

var script = loadTextContent('devVmInstall.sh')

resource vm 'Microsoft.Compute/virtualMachines@2023-07-01' existing = {
  name: vmName
}

resource devVmInstall 'Microsoft.Compute/virtualMachines/runCommands@2023-03-01' = {
  parent: vm
  name: 'devVmInstall'
  location: location
  properties: {
    asyncExecution: false
    outputBlobUri: '${logsUri}/devVm_install_stdout.txt'
    errorBlobUri: '${logsUri}/devVm_install_stderr.txt'
    source: {
      script: script
    }
  }
}
