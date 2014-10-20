# -*- mode: ruby -*-
# vi: set ft=ruby :

# Vagrantfile API/syntax version. Don't touch unless you know what you're doing!
VAGRANTFILE_API_VERSION = "2"

# Require a recent version of vagrant otherwise some have reported errors setting host names on boxes
Vagrant.require_version ">= 1.6.2"

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|

  if ENV['OPENSHIFT_DEV_CLUSTER']
    # Start an OpenShift cluster
    # The number of minions to provision
    num_minion = (ENV['OPENSHIFT_NUM_MINIONS'] || 2).to_i

    # IP configuration
    master_ip = "10.245.1.2"
    minion_ip_base = "10.245.2."
    minion_ips = num_minion.times.collect { |n| minion_ip_base + "#{n+2}" }
    minion_ips_str = minion_ips.join(",")

    # Determine the OS platform to use
    kube_os = ENV['OPENSHIFT_OS'] || "fedora"

    # OS platform to box information
    kube_box = {
      "fedora" => {
        "name" => "fedora20",
        "box_url" => "http://opscode-vm-bento.s3.amazonaws.com/vagrant/virtualbox/opscode_fedora-20_chef-provisionerless.box"
      }
    }

    # OpenShift master
    config.vm.define "master" do |config|
      config.vm.box = kube_box[kube_os]["name"]
      config.vm.box_url = kube_box[kube_os]["box_url"]
      config.vm.provision "shell", inline: "/vagrant/vagrant/provision-master.sh #{master_ip} #{num_minion} #{minion_ips_str}"
      config.vm.network "private_network", ip: "#{master_ip}"
      config.vm.hostname = "openshift-master"
    end

    # OpenShift minion
    num_minion.times do |n|
      config.vm.define "minion-#{n+1}" do |minion|
        minion_index = n+1
        minion_ip = minion_ips[n]
        minion.vm.box = kube_box[kube_os]["name"]
        minion.vm.box_url = kube_box[kube_os]["box_url"]
        minion.vm.provision "shell", inline: "/vagrant/vagrant/provision-minion.sh #{master_ip} #{num_minion} #{minion_ips_str} #{minion_ip} #{minion_index}"
        minion.vm.network "private_network", ip: "#{minion_ip}"
        minion.vm.hostname = "openshift-minion-#{minion_index}"
      end
    end
  else
    # Single VM dev environment
    # Set VirtualBox provider settings
    config.vm.provider "virtualbox" do |v, override|
      override.vm.box = "fedora20-virtualbox"
      override.vm.box_url = "http://opscode-vm-bento.s3.amazonaws.com/vagrant/virtualbox/opscode_fedora-20_chef-provisionerless.box"
      v.memory = 1024
      v.cpus = 2
      v.customize ["modifyvm", :id, "--cpus", "2"]
    end

    # Set VMware Fusion provider settings
    config.vm.provider "vmware_fusion" do |v, override|
      override.vm.box = "fedora20-vmware"
      override.vm.box_url = "http://opscode-vm-bento.s3.amazonaws.com/vagrant/vmware/opscode_fedora-20_chef-provisionerless.box"
      v.vmx["memsize"] = "1024"
      v.vmx["numvcpus"] = "2"
      v.gui = false
    end

    config.vm.define "openshiftdev", primary: true do |config|
      config.vm.hostname = "openshiftdev.local"

      if ENV['REBUILD_YUM_CACHE'] && ENV['REBUILD_YUM_CACHE'] != ""
        config.vm.provision "shell", inline: "yum clean all && yum makecache"
      end
      config.vm.provision "shell", path: "hack/vm-provision.sh"
      config.vm.synced_folder ENV["VAGRANT_SYNC_FROM"] || '.', ENV["VAGRANT_SYNC_TO"] || "/home/vagrant/go/src/github.com/openshift/origin"
    end
  end

end
