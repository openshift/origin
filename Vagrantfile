# -*- mode: ruby -*-
# vi: set ft=ruby :

# Vagrantfile API/syntax version. Don't touch unless you know what you're doing!
VAGRANTFILE_API_VERSION = "2"

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
  config.vm.define "openshiftdev", primary: true do |config|
    config.vm.box = "fedora20"
    config.vm.box_url = "http://opscode-vm-bento.s3.amazonaws.com/vagrant/virtualbox/opscode_fedora-20_chef-provisionerless.box"
    config.vm.hostname = "openshiftdev.local"
    config.vm.network "private_network", type: "dhcp"

    if ENV['REBUILD_YUM_CACHE'] && ENV['REBUILD_YUM_CACHE'] != ""
      config.vm.provision "shell", inline: "yum clean all && yum makecache"
    end

    config.vm.provision "shell", path: "provision.sh"
    config.vm.synced_folder '.', "/vagrant"
  end

  config.vm.provider "virtualbox" do |v|
    v.memory = 1024
    v.cpus = 2
    v.customize ["modifyvm", :id, "--cpus", "2"]
  end
end
