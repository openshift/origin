package kubevirt

import (
	"bytes"
	"text/template"
)

const (
	FedoraVMWithSecondaryNetworkAttachment = `
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: {{ .VMName }}
  namespace: {{ .VMNamespace }}
spec:
  running: true
  template:
    spec:
      domain:
        devices:
          disks:
            - name: containerdisk
              disk:
                bus: virtio
            - name: cloudinitdisk
              disk:
                bus: virtio
          interfaces:
          - name: underlay
            bridge: {}
        machine:
          type: ""
        resources:
          requests:
            memory: 2048M
      networks:
      - name: underlay
        multus:
          networkName: {{ .VMNamespace }}/{{ .NetworkName }}
      terminationGracePeriodSeconds: 0
      volumes:
        - name: containerdisk
          containerDisk:
            image: {{ .FedoraContainterDiskImage }} 
        - name: cloudinitdisk
          cloudInitNoCloud:
            networkData: |
              version: 2                                                              
              ethernets:                                                              
                eth0:                                                                 
                  dhcp4: true                                                         
                  dhcp6: true                                                         
            userData: |-
              #cloud-config
              password: fedora
              chpasswd: { expire: False }
`

	FedoraVMWithPrimaryUDNAttachment = `
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: {{ .VMName }}
  namespace: {{ .VMNamespace }}
spec:
  running: true
  template:
    spec:
      domain:
        devices:
          disks:
            - name: containerdisk
              disk:
                bus: virtio
            - name: cloudinitdisk
              disk:
                bus: virtio
          interfaces:
          - name: overlay
            binding:
              name: {{ .NetBindingName }}
        machine:
          type: ""
        resources:
          requests:
            memory: 2048M
      networks:
      - name: overlay
        pod: {}
      terminationGracePeriodSeconds: 0
      volumes:
        - name: containerdisk
          containerDisk:
            image: {{ .FedoraContainterDiskImage }}
        - name: cloudinitdisk
          cloudInitNoCloud:
            networkData: |
              version: 2                                                              
              ethernets:                                                              
                eth0:                                                                 
                  dhcp4: true                                                         
                  dhcp6: true                                                         
            userData: |-
              #cloud-config
              password: fedora
              chpasswd: { expire: False }
`

	FedoraVMIWithSecondaryNetworkAttachment = `
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstance
metadata:
  name: {{ .VMName }}
  namespace: {{ .VMNamespace }}
spec:
  domain:
    devices:
      disks:
      - disk:
          bus: virtio
        name: containerdisk
      - disk:
          bus: virtio
        name: cloudinitdisk
      interfaces:
      - bridge: {}
        name: overlay
      rng: {}
    resources:
      requests:
        memory: 2048M
  networks:
  - multus:
      networkName: {{ .VMNamespace }}/{{ .NetworkName }}
    name: overlay
  terminationGracePeriodSeconds: 0
  volumes:
  - containerDisk:
      image: {{ .FedoraContainterDiskImage }}
    name: containerdisk
  - cloudInitNoCloud:
      networkData: |
        version: 2                                                              
        ethernets:                                                              
          eth0:                                                                 
            dhcp4: true                                                         
            dhcp6: true                                                         
      userData: |-
        #cloud-config
        password: fedora
        chpasswd: { expire: False }
    name: cloudinitdisk
`

	FedoraVMIWithPrimaryUDNAttachment = `
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstance
metadata:
  name: {{ .VMName }}
  namespace: {{ .VMNamespace }}
spec:
  domain:
    devices:
      disks:
      - disk:
          bus: virtio
        name: containerdisk
      - disk:
          bus: virtio
        name: cloudinitdisk
      interfaces:
      - name: overlay
        binding:
          name: {{ .NetBindingName }}
      rng: {}
    resources:
      requests:
        memory: 2048M
  networks:
  - pod: {}
    name: overlay
  terminationGracePeriodSeconds: 0
  volumes:
  - containerDisk:
      image: {{ .FedoraContainterDiskImage }}
    name: containerdisk
  - cloudInitNoCloud:
      networkData: |
        version: 2                                                              
        ethernets:                                                              
          eth0:                                                                 
            dhcp4: true                                                         
            dhcp6: true                                                         
      userData: |-
        #cloud-config
        password: fedora
        chpasswd: { expire: False }
    name: cloudinitdisk
`
	FedoraVMWithPreconfiguredPrimaryUDNAttachment = `
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: {{ .VMName }}
  namespace: {{ .VMNamespace }}
spec:
  runStrategy: Always
  template:
    {{- if .PreconfiguredIP }}
    metadata:
      annotations:
        network.kubevirt.io/addresses: {{ printf "%q" .PreconfiguredIP }}
    {{- end }}
    spec:
      domain:
        devices:
          disks:
            - name: containerdisk
              disk:
                bus: virtio
            - name: cloudinitdisk
              disk:
                bus: virtio
          interfaces:
          - name: overlay
            binding:
              name: {{ .NetBindingName }}
        machine:
          type: ""
        resources:
          requests:
            memory: 2048M
      networks:
      - name: overlay
        pod: {}
      terminationGracePeriodSeconds: 0
      volumes:
        - name: containerdisk
          containerDisk:
            image: {{ .FedoraContainterDiskImage }}
        - name: cloudinitdisk
          cloudInitNoCloud:
            networkData: |
              version: 2                                                              
              ethernets:                                                              
                eth0:                                                                 
                  dhcp4: true                                                         
                  dhcp6: true                                                         
            userData: |-
              #cloud-config
              password: fedora
              chpasswd: { expire: False }
`
	vmimTemplate = `
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstanceMigration
metadata:
  namespace: {{ .VMNamespace }} 
  name: {{ .VMName }} 
spec:
  vmiName: {{ .VMName }} 
`
)

type CreationTemplateParams struct {
	VMName                    string
	VMNamespace               string
	FedoraContainterDiskImage string
	NetBindingName            string
	NetworkName               string
	PreconfiguredIP           string
}

func renderVMTemplate(vmTemplateString string, params CreationTemplateParams) (string, error) {
	vmTemplate, err := template.New(params.VMNamespace + "/" + params.VMName).Parse(vmTemplateString)
	if err != nil {
		return "", err
	}
	var vmResource bytes.Buffer
	if err := vmTemplate.Execute(&vmResource, params); err != nil {
		return "", err
	}
	return vmResource.String(), nil
}
