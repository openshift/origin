# -*- mode: ruby -*-
# vi: set ft=ruby :

# Vagrantfile API/syntax version. Don't touch unless you know what you're doing!
VAGRANTFILE_API_VERSION = "2"

# Require a recent version of vagrant otherwise some have reported errors setting host names on boxes
Vagrant.require_version ">= 1.6.2"

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|

  if File.exist?('.vagrant-openshift.json')
    json = File.read('.vagrant-openshift.json')
    vagrant_openshift_config = JSON.parse(json)
  else
    vagrant_openshift_config = {
      "instance_name" => "origin-dev",
      "os" => "fedora",
      "dev_cluster" => false,
      "num_minions" => 2,
      "rebuild_yum_cache" => false,
      "virtualbox" => {
        "box_name" => "fedora_inst",
        "box_url" => "https://mirror.openshift.com/pub/vagrant/boxes/openshift3/fedora_20_virtualbox_inst.box"
      },
      "vmware" => {
        "box_name" => "fedora_inst",
        "box_url" => "http://opscode-vm-bento.s3.amazonaws.com/vagrant/vmware/opscode_fedora-20_chef-provisionerless.box"
      },
      "aws" => {
        "ami" => "<AMI>",
        "ami_region" => "<AMI_REGION>",
        "ssh_user" => "<SSH_USER>",
        "machine_name" => "<AMI_NAME>"
      }
    }
  end


  if vagrant_openshift_config['dev_cluster'] || ENV['OPENSHIFT_DEV_CLUSTER']
    # Start an OpenShift cluster
    # The number of minions to provision
    num_minion = (vagrant_openshift_config['num_minions'] || ENV['OPENSHIFT_NUM_MINIONS'] || 2).to_i

    # IP configuration
    master_ip = "10.245.1.2"
    minion_ip_base = "10.245.2."
    minion_ips = num_minion.times.collect { |n| minion_ip_base + "#{n+2}" }
    minion_ips_str = minion_ips.join(",")

    # Determine the OS platform to use
    kube_os = vagrant_openshift_config['os'] || "fedora"

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
    sync_from = vagrant_openshift_config['sync_from'] || ENV["VAGRANT_SYNC_FROM"] || '.'
    sync_to = vagrant_openshift_config['sync_to'] || ENV["VAGRANT_SYNC_TO"] || "/data/src/github.com/openshift/origin"

    # Single VM dev environment
    # Set VirtualBox provider settings
    config.vm.provider "virtualbox" do |v, override|
      override.vm.box = vagrant_openshift_config['virtualbox']['box_name']
      override.vm.box_url = vagrant_openshift_config['virtualbox']['box_url']
      v.memory = 1024
      v.cpus = 2
      v.customize ["modifyvm", :id, "--cpus", "2"]
    end

    # Set VMware Fusion provider settings
    config.vm.provider "vmware_fusion" do |v, override|
      override.vm.box = vagrant_openshift_config['vmware']['box_name']
      override.vm.box_url = vagrant_openshift_config['vmware']['box_url']
      override.vm.provision "shell", path: "hack/vm-provision-vmware.sh", id: "setup"
      v.vmx["memsize"] = "1024"
      v.vmx["numvcpus"] = "2"
      v.gui = false
    end

    # Set AWS provider settings
    config.vm.provider :aws do |aws, override|
      creds_file_path = ENV['AWS_CREDS'].nil? || ENV['AWS_CREDS'] == '' ? "~/.awscred" : ENV['AWS_CREDS']
      if File.exist?(File.expand_path(creds_file_path))
        aws_creds_file = Pathname.new(File.expand_path(creds_file_path))
        aws_creds      = aws_creds_file.exist? ? Hash[*(File.open(aws_creds_file.to_s).readlines.map{ |l| l.split('=') }.flatten.map{ |i| i.strip })] : {}

        override.vm.box               = "dummy"
        override.vm.box_url           = "https://github.com/mitchellh/vagrant-aws/raw/master/dummy.box"
        override.vm.synced_folder sync_from, sync_to, disabled: true
        override.ssh.username         = vagrant_openshift_config['aws']['ssh_user']
        override.ssh.private_key_path = aws_creds["AWSPrivateKeyPath"] || "PATH TO AWS KEYPAIR PRIVATE KEY"

        aws.access_key_id     = aws_creds["AWSAccessKeyId"] || "AWS ACCESS KEY"
        aws.secret_access_key = aws_creds["AWSSecretKey"]   || "AWS SECRET KEY"
        aws.keypair_name      = aws_creds["AWSKeyPairName"] || "AWS KEYPAIR NAME"
        aws.ami               = vagrant_openshift_config['aws']['ami']
        aws.region            = vagrant_openshift_config['aws']['ami_region']
        aws.instance_type     = "m3.large"
        aws.instance_ready_timeout = 240
        aws.tags              = { "Name" => vagrant_openshift_config['instance_name'] }
        aws.user_data         = %{
#cloud-config

growpart:
  mode: auto
  devices: ['/']
runcmd:
- [ sh, -xc, "echo 'Defaults:#{vagrant_openshift_config['aws']['ssh_user']} \!requiretty' >> /etc/sudoers"]
        }
        aws.block_device_mapping = [
          {
             "DeviceName" => "/dev/sda1",
             "Ebs.VolumeSize" => 25,
             "Ebs.VolumeType" => "standard"
          }
        ]
        end
    end

    config.vm.define "openshiftdev", primary: true do |config|
      config.vm.hostname = "openshiftdev.local"

      if vagrant_openshift_config['rebuild_yum_cache']
        config.vm.provision "shell", inline: "yum clean all && yum makecache"
      end
      config.vm.provision "shell", path: "hack/vm-provision.sh", id: "setup"
      config.vm.synced_folder ".", "/vagrant", disabled: true
      config.vm.synced_folder sync_from, sync_to
    end
  end

end
