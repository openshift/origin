# -*- mode: ruby -*-
# vi: set ft=ruby :

# Vagrantfile API/syntax version. Don't touch unless you know what you're doing!
VAGRANTFILE_API_VERSION = "2"

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|

  # Set VirtualBox provider settings"  
  config.vm.provider "virtualbox" do |v, override|
    override.vm.box = "fedora20-virtualbox"
    override.vm.box_url = "http://opscode-vm-bento.s3.amazonaws.com/vagrant/virtualbox/opscode_fedora-20_chef-provisionerless.box"
    v.memory = 1024
    v.cpus = 2
    v.customize ["modifyvm", :id, "--cpus", "2"]
  end
  
  # Set VMware Fusion provider settings"  
  config.vm.provider "vmware_fusion" do |v, override|
    override.vm.box = "fedora20-vmware"
    override.vm.box_url = "http://opscode-vm-bento.s3.amazonaws.com/vagrant/vmware/opscode_fedora-20_chef-provisionerless.box"
    v.vmx["memsize"] = "1024"
    v.vmx["numvcpus"] = "2"
    v.gui = false
  end

  config.vm.define "openshiftdev", primary: true do |config|
    config.vm.hostname = "openshiftdev.local"
    #config.vm.network "private_network", type: "dhcp"

    if ENV['REBUILD_YUM_CACHE'] && ENV['REBUILD_YUM_CACHE'] != ""
      config.vm.provision "shell", inline: "yum clean all && yum makecache"
    end

    config.vm.provision "shell", path: "hack/vm-provision.sh"
    config.vm.synced_folder '.', "/home/vagrant/go/src/github.com/openshift/origin"
  end

end
